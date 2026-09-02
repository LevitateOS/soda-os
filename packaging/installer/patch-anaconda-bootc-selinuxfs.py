#!/usr/bin/python3
"""Apply Soda's exact Fedora 44 Anaconda bootc SELinuxFS correction."""

import hashlib
import pathlib
import subprocess


ANACONDA_CORE_NEVR = "anaconda-core-0:44.30-2.fc44"
SOURCE_SHA256 = "614ac3f3061d959144e0a2e80919012c7254d44b1fab04daea35b2bef52f3f86"
PATCHED_SHA256 = "de1400f91d39bcdba5f34d17b4173ef779c9d890e3ac404565d0c781026163de"
TARGET = pathlib.Path(
    "/usr/lib64/python3.14/site-packages/pyanaconda/modules/payloads/"
    "payload/rpm_ostree/installation.py"
)
OLD = b'        for path in ("/proc", "/sys"):\n'
NEW = b'        for path in ("/proc", "/sys", "/sys/fs/selinux"):\n'


def _sha256(contents):
    return hashlib.sha256(contents).hexdigest()


def main():
    installed_nevr = subprocess.run(
        [
            "/usr/bin/rpm",
            "-q",
            "--qf",
            "%{NAME}-%{EPOCHNUM}:%{VERSION}-%{RELEASE}",
            "anaconda-core",
        ],
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    ).stdout
    if installed_nevr != ANACONDA_CORE_NEVR:
        raise RuntimeError(
            f"unsupported Anaconda core for bootc SELinuxFS correction: "
            f"{installed_nevr}"
        )

    source = TARGET.read_bytes()
    if _sha256(source) != SOURCE_SHA256 or source.count(OLD) != 1:
        raise RuntimeError("Anaconda bootc mount source differs from the reviewed input")

    patched = source.replace(OLD, NEW)
    if _sha256(patched) != PATCHED_SHA256:
        raise RuntimeError("Anaconda bootc SELinuxFS correction differs from expectation")
    TARGET.write_bytes(patched)


if __name__ == "__main__":
    main()
