#!/usr/bin/env python3
"""Isolated matching-native Forgejo/PAM acceptance; see tests/acceptance/README.md.

Run as root through unshare --mount --net --pid --fork --kill-child --mount-proc
--propagation private. All accounts, credentials, databases and mounts are fixtures.
No real Forgejo account, Linux account, service, or registration setting is changed.
"""
import argparse
import atexit
import base64
import concurrent.futures
import grp
import http.cookiejar
import json
import os
import pathlib
import pwd
import secrets
import shutil
import sqlite3
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', default='/usr/bin/forgejo', help='Matching-native candidate executable')
parser.add_argument('--custom', default='/usr/share/soda/forgejo/custom', help='Matching candidate custom tree')
args = parser.parse_args()
assert os.geteuid() == 0 and os.getpid() == 1, 'Run in the documented private namespaces, never directly on the host'
work = pathlib.Path(tempfile.mkdtemp(prefix='soda-owner-acceptance-'))
atexit.register(shutil.rmtree, work)
git = pwd.getpwnam('git')
shadow_gid = grp.getgrnam('soda-forgejo-shadow').gr_gid
work.chmod(0o750)
os.chown(work, 0, git.pw_gid)
shutil.copyfile(args.binary, work / 'forgejo')
(work / 'forgejo').chmod(0o755)
subprocess.run(['chcon', '--reference=/usr/bin/forgejo', str(work / 'forgejo')], check=True)
subprocess.run(['mount', '--bind', str(work / 'forgejo'), '/usr/bin/forgejo'], check=True)
shutil.copytree(args.custom, work / 'custom')
for file in (work / 'custom').rglob('*'):
    file.chmod(0o755 if file.is_dir() else 0o644)
(work / 'custom').chmod(0o755)

linux_users = ['reviewowner', 'reviewpeer', 'reviewworkspace', 'reviewlate']
passwords = {name: secrets.token_urlsafe(24) for name in linux_users}
registration_passwords = {name: secrets.token_urlsafe(24) for name in [*linux_users, 'race-a', 'race-b']}
passwd = subprocess.check_output(['getent', 'passwd'], text=True)
groups = subprocess.check_output(['getent', 'group'], text=True)
used_ids = {int(line.split(':')[2]) for line in (passwd + groups).splitlines()}
uid = 24000
for name in linux_users:
    assert name not in passwd, 'Fixture username already exists'
    uid += 1
    while uid in used_ids:
        uid += 1
    passwd += f'{name}:x:{uid}:{uid}:Disposable owner test:{work}/{name}:/bin/bash\n'
    groups += f'{name}:x:{uid}:\n'
group_lines = []
for line in groups.splitlines():
    fields = line.split(':')
    if fields[0] in ['wheel', 'soda-workspaces']:
        name = 'reviewowner' if fields[0] == 'wheel' else 'reviewworkspace'
        fields[3] = ','.join(filter(None, [fields[3], name]))
    group_lines.append(':'.join(fields))
shadow = 'root:!:20000:0:99999:7:::\n'
for name in linux_users:
    digest = subprocess.check_output(['openssl', 'passwd', '-6', '-stdin'], input=passwords[name] + '\n', text=True).strip()
    shadow += f'{name}:{digest}:20000:0:99999:7:::\n'
for name, content in [('passwd', passwd), ('group', '\n'.join(group_lines) + '\n'), ('shadow', shadow)]:
    file = work / name
    file.write_text(content)
    file.chmod(0o040 if name == 'shadow' else 0o644)
    if name == 'shadow':
        os.chown(file, 0, shadow_gid)
    subprocess.run(['chcon', '--reference=/etc/' + name, str(file)], check=True)
    subprocess.run(['mount', '--bind', str(file), '/etc/' + name], check=True)
subprocess.run(['ip', 'link', 'set', 'lo', 'up'], check=True)
print('Native platform:', os.uname().machine, flush=True)


class Fixture:
    def __init__(self, name, active=True, disabled=False):
        self.path = work / name
        self.path.mkdir(mode=0o700)
        os.chown(self.path, git.pw_uid, git.pw_gid)
        self.config = self.path / 'app.ini'
        self.base = 'http://127.0.0.1:34567'
        self.config.write_text(f'''APP_NAME = Soda OS
RUN_MODE = prod
RUN_USER = git
WORK_PATH = {self.path}
[database]
DB_TYPE = sqlite3
PATH = {self.path}/data/forgejo.db
[repository]
ROOT = {self.path}/repositories
[server]
APP_DATA_PATH = {self.path}/data
HTTP_ADDR = 127.0.0.1
HTTP_PORT = 34567
ROOT_URL = {self.base}/
DISABLE_SSH = true
[service]
DISABLE_REGISTRATION = {str(disabled).lower()}
[security]
INSTALL_LOCK = true
SECRET_KEY = {secrets.token_hex(32)}
INTERNAL_TOKEN = {secrets.token_hex(64)}
PASSWORD_CHECK_PWN = false
[session]
PROVIDER = file
PROVIDER_CONFIG = {self.path}/sessions
[log]
MODE = console
LEVEL = Warn
ROOT_PATH = {self.path}/log
[cron]
ENABLED = false
[mailer]
ENABLED = false
''')
        self.config.chmod(0o600)
        os.chown(self.config, git.pw_uid, git.pw_gid)
        self.env = dict(os.environ, HOME=str(self.path), USER='git', FORGEJO_WORK_DIR=str(self.path), FORGEJO_CUSTOM=str(work / 'custom'))
        self.cli('migrate')
        self.cli('admin', 'auth', 'add-pam', '--name', 'Soda OS', '--service-name', 'soda-forgejo', '--active=' + str(active).lower())
        self.server = None
        self.log = None
        self.start()

    def cli(self, *args):
        result = subprocess.run(['/usr/bin/forgejo', *args, '--config', str(self.config)], env=self.env, cwd=self.path, user=git.pw_uid, group=git.pw_gid, extra_groups=[shadow_gid], capture_output=True)
        assert result.returncode == 0, 'Fixture CLI failed'

    def start(self):
        self.log = open(self.path / 'server.log', 'ab')
        self.server = subprocess.Popen(['/usr/bin/forgejo', 'web', '--config', str(self.config)], env=self.env, cwd=self.path, user=git.pw_uid, group=git.pw_gid, extra_groups=[shadow_gid], stdout=self.log, stderr=subprocess.STDOUT)
        for _ in range(150):
            try:
                with urllib.request.urlopen(self.base + '/api/healthz', timeout=1) as response:
                    if response.status == 200:
                        return
            except OSError:
                pass
            if self.server.poll() is not None:
                break
            time.sleep(.1)
        self.close()
        raise RuntimeError('Fixture did not become ready')

    def close(self):
        if self.server and self.server.poll() is None:
            self.server.terminate()
            self.server.wait(timeout=20)
        if self.log:
            self.log.close()

    def rows(self):
        with sqlite3.connect('file:' + str(self.path / 'data/forgejo.db') + '?mode=ro', uri=True) as db:
            return list(db.execute('SELECT id,name,login_type,is_admin FROM "user" WHERE id>0 ORDER BY id'))

    def get(self, route, name=None, password=None):
        headers = {}
        if name:
            headers['Authorization'] = 'Basic ' + base64.b64encode((name + ':' + (password or passwords[name])).encode()).decode()
        try:
            with urllib.request.urlopen(urllib.request.Request(self.base + route, headers=headers)) as response:
                return response.status, response.geturl(), response.read().decode()
        except urllib.error.HTTPError as error:
            return error.code, error.geturl(), error.read().decode()

    def form(self, route, fields):
        opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        with opener.open(self.base + route) as response:
            response.read()
        request = urllib.request.Request(self.base + route, data=urllib.parse.urlencode(fields).encode(), headers={'Origin': self.base, 'Referer': self.base + route})
        try:
            with opener.open(request) as response:
                return response.status, response.geturl(), response.read().decode()
        except urllib.error.HTTPError as error:
            return error.code, error.geturl(), error.read().decode()

    def signup(self, name, password=None):
        password = password or registration_passwords[name]
        return self.form('/user/sign_up', {'user_name': name, 'email': name + '@example.test', 'password': password, 'retype': password})

    def report(self, message):
        print(self.path.name + ': ' + message + '; ' + json.dumps(self.rows()), flush=True)


fixture = None
try:
    fixture = Fixture('normal')
    assert 'Create your Forgejo administrator account' in fixture.get('/')[2]
    assert fixture.get('/user/login')[1].endswith('/user/sign_up')
    for route in ['/api/v1/user', '/reviewowner/test.git/info/refs?service=git-upload-pack']:
        status, _, body = fixture.get(route, 'reviewowner')
        assert status == 401 and 'first-owner registration' in body
    status, url, _ = fixture.form('/user/login', {'user_name': 'reviewowner', 'password': passwords['reviewowner']})
    assert status == 200 and url.endswith('/user/sign_up') and fixture.rows() == []
    fixture.signup('reviewowner', 'short')
    assert fixture.rows() == [], 'Failed registration must remain retryable'
    status, url, body = fixture.signup('reviewowner')
    assert status == 200 and url.endswith('/admin') and 'Your Forgejo administrator account is ready' in body
    assert fixture.rows() == [(1, 'reviewowner', 0, 1)]
    assert fixture.get('/api/v1/user', 'reviewowner')[0] == 401
    assert fixture.get('/api/v1/user', 'reviewowner', registration_passwords['reviewowner'])[0] == 200
    assert fixture.get('/api/v1/user', 'reviewpeer')[0] == 200
    assert fixture.rows()[1][2:] == (4, 0)
    assert fixture.get('/api/v1/user', 'reviewworkspace')[0] == 401
    with sqlite3.connect('file:' + str(fixture.path / 'data/forgejo.db') + '?mode=ro', uri=True) as db:
        assert db.execute('SELECT length(passwd),length(salt),length(passwd_hash_algo) FROM "user" WHERE name=?', ('reviewpeer',)).fetchone() == (0, 0, 0)
    fixture.report('early web/API/Git requests blocked; owner registered atomically; independent credentials; ordinary peer; workspace exclusion; no retained PAM verifier')
    fixture.close()
    fixture.start()
    assert fixture.get('/api/v1/user', 'reviewowner', registration_passwords['reviewowner'])[0] == 200
    assert fixture.rows()[0][3] == 1 and fixture.rows()[1][3] == 0
    group_file = work / 'group'
    group_file.write_text('\n'.join(line + ',reviewpeer' if line.startswith('wheel:') else line for line in group_file.read_text().splitlines()) + '\n')
    assert 'wheel' in subprocess.check_output(['id', '-Gn', 'reviewpeer'], text=True)
    assert json.loads(fixture.get('/api/v1/user', 'reviewpeer')[2])['is_admin'] is False
    fixture.report('roles survive process restart and later Linux wheel promotion')
    # Inject the known historical failure into THIS disposable database only.
    with sqlite3.connect(fixture.path / 'data/forgejo.db') as db:
        db.execute('UPDATE "user" SET is_admin=0 WHERE id=1')
    assert 'This Forgejo instance has no administrator' in fixture.get('/')[2]
    count = len(fixture.rows())
    fixture.signup('race-a')
    assert len(fixture.rows()) == count and all(row[3] == 0 for row in fixture.rows())
    status, _, body = fixture.get('/api/v1/user', 'reviewlate')
    assert status == 401 and 'first-owner registration' in body and len(fixture.rows()) == count
    assert fixture.get('/api/v1/user', 'reviewpeer')[0] == 200
    status, _, body = fixture.form('/user/login', {'user_name': 'reviewpeer', 'password': passwords['reviewpeer']})
    assert status == 200 and 'This Forgejo instance has no administrator' not in body, 'Keep the existing authenticated dashboard'
    fixture.report('existing admin-less state diagnosed without promotion, deletion, or blocking existing PAM accounts and dashboards')
    fixture.close()

    fixture = Fixture('inactive-source', active=False)
    fixture.signup('reviewowner')
    assert fixture.rows()[0][3] == 1
    assert fixture.get('/api/v1/user', 'reviewpeer')[0] == 401
    fixture.close()
    fixture.start()
    assert fixture.get('/api/v1/user', 'reviewpeer')[0] == 401
    fixture.cli('admin', 'auth', 'update-pam', '--id', '1', '--active=true')
    assert fixture.get('/api/v1/user', 'reviewpeer')[0] == 200 and fixture.rows()[1][3] == 0
    fixture.report('administrator activation choice survives bootstrap and restart; native explicit activation works')
    fixture.close()

    fixture = Fixture('registration-policy', disabled=True)
    assert fixture.get('/api/v1/user', 'reviewowner')[0] == 401
    assert 'Local registration is disabled' in fixture.get('/')[2]
    assert fixture.signup('reviewowner')[0] == 403 and fixture.rows() == []
    fixture.close()
    fixture.config.write_text(fixture.config.read_text().replace('DISABLE_REGISTRATION = true', 'DISABLE_REGISTRATION = false'))
    fixture.start()
    fixture.signup('reviewowner')
    fixture.close()
    fixture.config.write_text(fixture.config.read_text().replace('DISABLE_REGISTRATION = false', 'DISABLE_REGISTRATION = true'))
    fixture.start()
    assert fixture.get('/api/v1/user', 'reviewpeer')[0] == 200 and fixture.rows()[1][3] == 0
    fixture.report('registration policy preserved before and after bootstrap; later PAM remains available')
    fixture.close()

    fixture = Fixture('concurrent-registration')
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        list(pool.map(fixture.signup, ['race-a', 'race-b']))
    assert sum(row[3] for row in fixture.rows()) == 1
    fixture.report('competing native registration requests leave exactly one administrator')
    fixture.close()
    fixture = None
    print('First-owner native acceptance passed. This does not establish OS reboot/image-update or browser usability acceptance.', flush=True)
finally:
    if fixture:
        fixture.close()
