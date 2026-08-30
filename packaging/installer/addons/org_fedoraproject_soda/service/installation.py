import json
import os
from pathlib import Path

from pyanaconda.modules.common.task import Task


class WriteInstallerPersonTask(Task):
    def __init__(self, sysroot, username, name, email):
        super().__init__()
        self._sysroot = sysroot
        self._identity = {"username": username, "name": name, "email": email}

    @property
    def name(self):
        return "Record the first Soda administrator"

    def run(self):
        state_dir = Path(self._sysroot) / "var/lib/soda"
        state_dir.mkdir(parents=True, exist_ok=True, mode=0o750)
        path = state_dir / "installer-admin.json"
        temporary = state_dir / ".installer-admin.json.tmp"
        temporary.write_text(json.dumps(self._identity) + "\n", encoding="utf-8")
        os.chmod(temporary, 0o600)
        temporary.replace(path)
