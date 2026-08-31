import fcntl
import http.client
import os
import subprocess
import time
import urllib.parse
from pathlib import Path, PurePosixPath

from pyanaconda.core.users import crypt_password
from pyanaconda.modules.common.constants.services import USERS
from pyanaconda.modules.common.structures.sshkey import SshKeyData
from pyanaconda.modules.common.structures.user import UserData
from pyanaconda.modules.common.task import Task
from pyanaconda.ui.lib.users import get_user_list, set_user_list


FORGEJO_CONFIG = "/etc/forgejo/app.ini"
FORGEJO_HOST = "127.0.0.1"
FORGEJO_PORT = 30000
TAILSCALE_KEY_PATH = "/var/lib/soda-install/tailscale-auth-key"


class ProvisionSodaInstallationTask(Task):
    def __init__(self, sysroot, tailscale_auth_key):
        super().__init__()
        self._sysroot = Path(sysroot)
        self._tailscale_auth_key = tailscale_auth_key
        self._users = USERS.get_proxy()

    @property
    def name(self):
        return "Provision the first Soda administrator"

    def run(self):
        passwords_replaced = False
        try:
            users = get_user_list(self._users)
            user, password = self._validate_native_input(users)
            self._validate_installed_linux_account(user)
            self._create_forgejo_administrator(user.name, password)
            self._replace_plaintext_passwords()
            passwords_replaced = True
            self._write_tailscale_auth_key()
        finally:
            try:
                if not passwords_replaced:
                    self._replace_plaintext_passwords()
            finally:
                self._tailscale_auth_key = ""

    def _validate_native_input(self, users):
        if len(users) != 1:
            raise RuntimeError("the installer requires exactly one administrator")

        user = users[0]
        if not user.name or not user.has_admin_priviledges():
            raise RuntimeError("the installer administrator must be a Linux administrator")
        if not user.password or user.is_crypted:
            raise RuntimeError("the installer requires the administrator's plaintext password")
        if not self._tailscale_auth_key:
            raise RuntimeError("the installer requires a one-use Tailscale auth key")

        keys = SshKeyData.from_structure_list(self._users.SshKeys)
        if (
            len(keys) != 1
            or keys[0].username != user.name
            or not keys[0].key.strip()
        ):
            raise RuntimeError("the installer requires exactly one administrator SSH key")

        return user, user.password

    def _validate_installed_linux_account(self, user):
        passwd_fields = self._find_account_record("etc/passwd", user.name, 7)
        expected_home = PurePosixPath("/home") / user.name
        if PurePosixPath(passwd_fields[5]) != expected_home:
            raise RuntimeError("the installed administrator has an unexpected home")

        wheel_fields = self._find_account_record("etc/group", "wheel", 4)
        wheel_members = {value for value in wheel_fields[3].split(",") if value}
        if user.name not in wheel_members:
            raise RuntimeError("the installed administrator is not a member of wheel")

        shadow_fields = self._find_account_record("etc/shadow", user.name, 2)
        password_hash = shadow_fields[1]
        if not password_hash or password_hash.startswith(("!", "*")):
            raise RuntimeError("the installed administrator password is not usable")

        keys = [
            key.key.strip()
            for key in SshKeyData.from_structure_list(self._users.SshKeys)
            if key.username == user.name
        ]
        authorized_keys = self._sysroot / str(expected_home).lstrip("/") / ".ssh/authorized_keys"
        try:
            installed_keys = {
                line.strip() for line in authorized_keys.read_text(encoding="utf-8").splitlines()
                if line.strip()
            }
        except OSError:
            raise RuntimeError("the installed administrator SSH key is missing") from None
        if len(keys) != 1 or keys[0] not in installed_keys:
            raise RuntimeError("the installed administrator SSH key does not match")

    def _find_account_record(self, relative_path, name, minimum_fields):
        try:
            lines = (self._sysroot / relative_path).read_text(encoding="utf-8").splitlines()
        except OSError:
            raise RuntimeError("the installed Linux account database is unavailable") from None

        matches = []
        for line in lines:
            fields = line.split(":")
            if fields and fields[0] == name:
                matches.append(fields)
        if len(matches) != 1 or len(matches[0]) < minimum_fields:
            raise RuntimeError("the installed Linux account is incomplete")
        return matches[0]

    def _create_forgejo_administrator(self, username, password):
        mounted_dev = False
        mounted_proc = False
        config_fd = None
        process = None
        try:
            mounted_dev = self._ensure_target_mount("/dev", "dev", "null")
            mounted_proc = self._ensure_target_mount("/proc", "proc", "self/fd")
            self._run_target_root([
                "/usr/bin/systemd-tmpfiles",
                "--create",
                "forgejo.conf",
            ])
            self._run_target_root(["/usr/libexec/soda/forgejo-init"])
            if self._forgejo_users(admin_only=False):
                raise RuntimeError("Forgejo already contains a user")

            config_path = self._sysroot / FORGEJO_CONFIG.lstrip("/")
            try:
                durable_config = config_path.read_bytes()
            except OSError:
                raise RuntimeError("Forgejo configuration was not initialized") from None
            transient_config = self._forgejo_bootstrap_config(durable_config)
            config_fd = self._sealed_memfd(transient_config)
            process = subprocess.Popen(
                self._target_user_command(
                    ["/usr/bin/forgejo", "web", "--config", f"/proc/self/fd/{config_fd}"]
                ),
                pass_fds=(config_fd,),
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                close_fds=True,
            )
            self._wait_for_forgejo(process)
            self._submit_forgejo_signup(username, password)
            self._stop_process(process)
            process = None
            try:
                if config_path.read_bytes() != durable_config:
                    raise RuntimeError("Forgejo changed its durable bootstrap configuration")
            except OSError:
                raise RuntimeError("Forgejo durable configuration is unavailable") from None

            administrators = self._forgejo_users(admin_only=True)
            if administrators != [(username, True, True)]:
                raise RuntimeError("Forgejo did not create the requested site administrator")
        finally:
            cleanup_failed = False
            try:
                self._stop_process(process)
            except Exception:
                cleanup_failed = True
            if config_fd is not None:
                try:
                    os.close(config_fd)
                except OSError:
                    cleanup_failed = True
            if mounted_proc:
                cleanup_failed = not self._unmount_target_mount("proc") or cleanup_failed
            if mounted_dev:
                cleanup_failed = not self._unmount_target_mount("dev") or cleanup_failed
            if cleanup_failed:
                raise RuntimeError("temporary Forgejo mounts could not be removed")

    @staticmethod
    def _forgejo_bootstrap_config(durable_config):
        required = {
            b"HTTP_ADDR = 127.0.0.1": 1,
            b"HTTP_PORT = 30000": 1,
            b"DISABLE_REGISTRATION = true": 1,
            b"INSTALL_LOCK = true": 1,
        }
        for setting, expected_count in required.items():
            if durable_config.count(setting) != expected_count:
                raise RuntimeError("Forgejo has an unexpected bootstrap configuration")
        return durable_config.replace(
            b"DISABLE_REGISTRATION = true",
            b"DISABLE_REGISTRATION = false",
            1,
        )

    @staticmethod
    def _sealed_memfd(contents):
        fd = os.memfd_create(
            "soda-forgejo-bootstrap",
            os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING,
        )
        try:
            with os.fdopen(os.dup(fd), "wb") as stream:
                stream.write(contents)
                stream.flush()
            os.lseek(fd, 0, os.SEEK_SET)
            seals = (
                fcntl.F_SEAL_SEAL
                | fcntl.F_SEAL_SHRINK
                | fcntl.F_SEAL_GROW
                | fcntl.F_SEAL_WRITE
            )
            fcntl.fcntl(fd, fcntl.F_ADD_SEALS, seals)
            return fd
        except Exception:
            os.close(fd)
            raise

    def _ensure_target_mount(self, source, relative_target, probe):
        target = self._sysroot / relative_target
        target.mkdir(parents=True, exist_ok=True)
        if (target / probe).exists():
            return False
        result = subprocess.run(
            ["/usr/bin/mount", "--bind", source, str(target)],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        if result.returncode != 0 or not (target / probe).exists():
            raise RuntimeError("a required target filesystem is unavailable")
        return True

    def _unmount_target_mount(self, relative_target):
        result = subprocess.run(
            ["/usr/bin/umount", str(self._sysroot / relative_target)],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        return result.returncode == 0

    def _target_user_command(self, command):
        return [
            "/usr/sbin/chroot",
            "--userspec=git:git",
            str(self._sysroot),
            "/usr/bin/env",
            "HOME=/var/lib/forgejo",
            "USER=git",
            "FORGEJO_WORK_DIR=/var/lib/forgejo",
            *command,
        ]

    def _run_target_root(self, command):
        result = subprocess.run(
            ["/usr/sbin/chroot", str(self._sysroot), *command],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError("Forgejo initialization failed")

    def _forgejo_users(self, admin_only):
        command = [
            "/usr/bin/forgejo",
            "admin",
            "user",
            "list",
            "--config",
            FORGEJO_CONFIG,
        ]
        if admin_only:
            command.append("--admin")
        result = subprocess.run(
            self._target_user_command(command),
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError("Forgejo user verification failed")
        users = []
        for line in result.stdout.splitlines():
            fields = line.split()
            if not fields or not fields[0].isdigit():
                continue
            if admin_only and len(fields) >= 4:
                users.append((
                    fields[1],
                    fields[3].lower() == "true",
                    True,
                ))
                continue
            if not admin_only and len(fields) >= 5:
                users.append((
                    fields[1],
                    fields[3].lower() == "true",
                    fields[4].lower() == "true",
                ))
                continue
            raise RuntimeError("Forgejo returned an invalid user list")
        return users

    @staticmethod
    def _wait_for_forgejo(process):
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            if process.poll() is not None:
                raise RuntimeError("Forgejo exited during administrator creation")
            connection = http.client.HTTPConnection(FORGEJO_HOST, FORGEJO_PORT, timeout=1)
            try:
                connection.request("GET", "/api/healthz")
                response = connection.getresponse()
                response.read()
                if response.status == 200:
                    return
            except OSError:
                pass
            finally:
                connection.close()
            time.sleep(0.2)
        raise RuntimeError("Forgejo did not become ready")

    @staticmethod
    def _submit_forgejo_signup(username, password):
        form = urllib.parse.urlencode({
            "user_name": username,
            "email": f"{username}@localhost",
            "password": password,
            "retype": password,
        }).encode("utf-8")
        connection = http.client.HTTPConnection(FORGEJO_HOST, FORGEJO_PORT, timeout=10)
        try:
            try:
                connection.request(
                    "POST",
                    "/user/sign_up",
                    body=form,
                    headers={
                        "Content-Type": "application/x-www-form-urlencoded",
                        "Content-Length": str(len(form)),
                        "Sec-Fetch-Site": "same-origin",
                    },
                )
                response = connection.getresponse()
                status = response.status
                location = response.getheader("Location", "")
                response.read()
            except Exception:
                raise RuntimeError("Forgejo rejected administrator creation") from None
        finally:
            connection.close()
        if status != 303 or location != "/":
            raise RuntimeError("Forgejo rejected administrator creation")

    @staticmethod
    def _stop_process(process):
        if process is None or process.poll() is not None:
            return
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)

    def _write_tailscale_auth_key(self):
        state_dir = self._sysroot / "var/lib/soda-install"
        state_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        os.chmod(state_dir, 0o700)
        destination = self._sysroot / TAILSCALE_KEY_PATH.lstrip("/")
        temporary = state_dir / ".tailscale-auth-key.tmp"
        if destination.exists() or temporary.exists():
            raise RuntimeError("the Tailscale enrollment handoff already exists")

        descriptor = os.open(
            temporary,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
            0o600,
        )
        try:
            with os.fdopen(descriptor, "wb") as stream:
                stream.write(self._tailscale_auth_key.encode("utf-8") + b"\n")
                stream.flush()
                os.fsync(stream.fileno())
            # Pinned Anaconda's later SetContextsTask relabels /var/lib in the
            # target. The atomic rename remains this task's final operation.
            os.replace(temporary, destination)
        except Exception:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass
            raise

    def _replace_plaintext_passwords(self):
        users = get_user_list(self._users)
        changed = False
        for user in users:
            if user.password and not user.is_crypted:
                user.password = crypt_password(user.password)
                user.is_crypted = True
                changed = True
        if changed:
            set_user_list(self._users, users)
