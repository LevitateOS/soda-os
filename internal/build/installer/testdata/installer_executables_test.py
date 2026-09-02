#!/usr/bin/python3

import importlib.machinery
import importlib.util
import os
import pathlib
import re
import stat
import subprocess
import sys
import tempfile
import types
import unittest
from unittest import mock


def load_executable(name, path):
    loader = importlib.machinery.SourceFileLoader(name, str(path))
    specification = importlib.util.spec_from_loader(name, loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


def write_text(path, contents, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(contents, encoding="utf-8")
    path.chmod(mode)


class InstallerExecutableTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.input = load_executable("soda_installer_input_test", INPUT_PATH)
        cls.finalizer = load_executable("soda_installer_finalize_test", FINALIZER_PATH)

    def make_public_key(self, directory):
        private_key = directory / "id_ed25519"
        subprocess.run(
            [
                "/usr/bin/ssh-keygen",
                "-q",
                "-t",
                "ed25519",
                "-N",
                "",
                "-f",
                str(private_key),
            ],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=True,
        )
        return private_key.with_suffix(".pub").read_text(encoding="utf-8").strip()

    def test_materialize_inputs_writes_canonical_account_kickstart(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            media = root / "media" / "soda"
            runtime = root / "runtime"
            media.mkdir(parents=True)
            public_key = self.make_public_key(root)
            key_fields = public_key.split()
            canonical_key = f"{key_fields[0]} {key_fields[1]}"
            password = "correct horse battery staple"

            write_text(media / self.input.USERNAME_FILE, "soda-test\n")
            write_text(media / self.input.PASSWORD_FILE, password + "\n")
            write_text(
                media / self.input.SSH_KEY_FILE,
                public_key + " installer-comment\n",
            )
            write_text(
                media / self.input.TAILSCALE_KEY_FILE,
                "tskey-auth-test-value\n",
            )

            with (
                mock.patch.object(self.input, "MEDIA_INPUT_DIR", media),
                mock.patch.object(self.input, "RUNTIME_DIR", runtime),
                mock.patch.object(
                    self.input,
                    "_password_hash",
                    return_value="$6$fixture$digest",
                ) as password_hash,
            ):
                self.input._materialize_inputs()

            password_hash.assert_called_once_with(password)

            self.assertEqual(stat.S_IMODE(runtime.stat().st_mode), 0o700)
            for name in (
                self.input.USERNAME_FILE,
                self.input.PASSWORD_FILE,
                self.input.SSH_KEY_FILE,
                self.input.TAILSCALE_KEY_FILE,
                self.input.ACCOUNT_KICKSTART,
            ):
                self.assertEqual(stat.S_IMODE((runtime / name).stat().st_mode), 0o600)

            self.assertEqual(
                (runtime / self.input.SSH_KEY_FILE).read_text(encoding="utf-8"),
                canonical_key + "\n",
            )
            account = (runtime / self.input.ACCOUNT_KICKSTART).read_text(
                encoding="utf-8"
            )
            match = re.fullmatch(
                'user --name=soda-test --groups=wheel --password="([^"\\s]+)" --iscrypted\\n'
                'sshkey --username=soda-test "([^"\\n]+)"\\n',
                account,
            )
            self.assertIsNotNone(match)
            self.assertTrue(match.group(1).startswith("$6$"))
            self.assertEqual(match.group(2), canonical_key)
            self.assertNotIn(password, account)
            self.assertNotIn("installer-comment", account)

    def test_runtime_file_is_removed_when_durable_write_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            runtime = pathlib.Path(temporary) / "runtime"
            runtime.mkdir(mode=0o700)
            with (
                mock.patch.object(self.input, "RUNTIME_DIR", runtime),
                mock.patch.object(
                    self.input.os, "fsync", side_effect=OSError("fsync failed")
                ),
            ):
                with self.assertRaisesRegex(OSError, "fsync failed"):
                    self.input._write_runtime_file("secret", b"value\n")
            self.assertFalse((runtime / "secret").exists())

    def test_persistent_var_mount_matches_resolved_sysroot_path(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            physical_root = root / "var" / "mnt" / "sysroot"
            target_var = physical_root / "var"
            target_var.mkdir(parents=True)
            (root / "mnt").symlink_to(root / "var" / "mnt", target_is_directory=True)
            sysroot_alias = root / "mnt" / "sysroot"
            mountinfo = root / "mountinfo"
            resolved_target_var = pathlib.Path(os.path.realpath(target_var))
            mountinfo.write_text(
                f"40 30 0:40 / {resolved_target_var} rw,relatime - ext4 /dev/test rw\n",
                encoding="utf-8",
            )

            path_type = pathlib.Path

            def test_path(value):
                if os.fspath(value) == "/proc/self/mountinfo":
                    return mountinfo
                return path_type(value)

            with (
                mock.patch.object(self.finalizer, "SYSROOT", sysroot_alias),
                mock.patch.object(self.finalizer, "Path", side_effect=test_path),
            ):
                self.finalizer._validate_persistent_target_var()

            mountinfo.write_text(
                f"40 30 0:40 / {sysroot_alias / 'var'} rw,relatime - ext4 /dev/test rw\n",
                encoding="utf-8",
            )
            with (
                mock.patch.object(self.finalizer, "SYSROOT", sysroot_alias),
                mock.patch.object(self.finalizer, "Path", side_effect=test_path),
            ):
                with self.assertRaisesRegex(
                    RuntimeError, "persistent bootc variable-data root is not mounted"
                ):
                    self.finalizer._validate_persistent_target_var()

    def test_target_api_filesystems_are_always_bind_mounted_and_cleaned_up(self):
        with tempfile.TemporaryDirectory() as temporary:
            sysroot = pathlib.Path(temporary) / "sysroot"
            (sysroot / "dev").mkdir(parents=True)
            (sysroot / "dev" / "null").touch()
            (sysroot / "proc").mkdir()
            (sysroot / "proc" / "self").mkdir()
            completed = types.SimpleNamespace(returncode=0)

            with (
                mock.patch.object(self.finalizer, "SYSROOT", sysroot),
                mock.patch.object(
                    self.finalizer.subprocess, "run", return_value=completed
                ) as run,
            ):
                self.finalizer._bind_target_mount("/dev", "dev")
                self.finalizer._bind_target_mount("/proc", "proc")

            self.assertEqual(
                [call.args[0] for call in run.call_args_list],
                [
                    ["/usr/bin/mount", "--bind", "/dev", str(sysroot / "dev")],
                    ["/usr/bin/mount", "--bind", "/proc", str(sysroot / "proc")],
                ],
            )

            with (
                mock.patch.object(
                    self.finalizer,
                    "_bind_target_mount",
                    side_effect=[True, True],
                ) as bind,
                mock.patch.object(
                    self.finalizer,
                    "_run_target_root",
                    side_effect=RuntimeError("fixture stop"),
                ),
                mock.patch.object(
                    self.finalizer, "_unmount_target", return_value=True
                ) as unmount,
            ):
                with self.assertRaisesRegex(RuntimeError, "fixture stop"):
                    self.finalizer._create_forgejo_administrator(
                        "soda-test", "password"
                    )

            self.assertEqual(
                bind.call_args_list,
                [mock.call("/dev", "dev"), mock.call("/proc", "proc")],
            )
            self.assertEqual(
                unmount.call_args_list,
                [mock.call("proc"), mock.call("dev")],
            )

    def test_installed_linux_account_validation(self):
        with tempfile.TemporaryDirectory() as temporary:
            sysroot = pathlib.Path(temporary) / "sysroot"
            public_key = self.make_public_key(pathlib.Path(temporary))
            fields = public_key.split()
            canonical_key = f"{fields[0]} {fields[1]}"
            passwd = sysroot / "etc/passwd"
            group = sysroot / "etc/group"
            shadow = sysroot / "etc/shadow"
            authorized_keys = sysroot / "home/soda-test/.ssh/authorized_keys"

            valid_passwd = "soda-test:x:1000:1000:Soda Test:/home/soda-test:/bin/bash\n"
            valid_group = "wheel:x:10:soda-test\n"
            valid_shadow = "soda-test:$6$salt$digest:1:0:99999:7:::\n"
            write_text(passwd, valid_passwd)
            write_text(group, valid_group)
            write_text(shadow, valid_shadow)
            write_text(authorized_keys, public_key + "\n")

            with mock.patch.object(self.finalizer, "SYSROOT", sysroot):
                self.finalizer._validate_installed_linux_account(
                    "soda-test", canonical_key
                )

                write_text(
                    passwd,
                    "soda-test:x:1000:1000:Soda Test:/var/home/soda-test:/bin/bash\n",
                )
                with self.assertRaisesRegex(RuntimeError, "unexpected home"):
                    self.finalizer._validate_installed_linux_account(
                        "soda-test", canonical_key
                    )
                write_text(passwd, valid_passwd)

                write_text(group, "wheel:x:10:\n")
                with self.assertRaisesRegex(RuntimeError, "member of wheel"):
                    self.finalizer._validate_installed_linux_account(
                        "soda-test", canonical_key
                    )
                write_text(group, valid_group)

                write_text(shadow, "soda-test:!locked:1:0:99999:7:::\n")
                with self.assertRaisesRegex(RuntimeError, "password is not usable"):
                    self.finalizer._validate_installed_linux_account(
                        "soda-test", canonical_key
                    )
                write_text(shadow, valid_shadow)

                write_text(authorized_keys, "ssh-ed25519 invalid\n")
                with self.assertRaisesRegex(RuntimeError, "does not match"):
                    self.finalizer._validate_installed_linux_account(
                        "soda-test", canonical_key
                    )

    def test_tailscale_handoff_is_atomic_and_mode_restricted(self):
        with tempfile.TemporaryDirectory() as temporary:
            sysroot = pathlib.Path(temporary) / "sysroot"
            sysroot.mkdir()
            state_dir = sysroot / "var/lib/soda-install"
            original_lstat = pathlib.Path.lstat
            original_replace = os.replace

            def root_owned_lstat(path):
                metadata = original_lstat(path)
                if path != state_dir:
                    return metadata
                return types.SimpleNamespace(
                    st_mode=metadata.st_mode,
                    st_uid=0,
                    st_gid=0,
                )

            with (
                mock.patch.object(self.finalizer, "SYSROOT", sysroot),
                mock.patch.object(pathlib.Path, "lstat", new=root_owned_lstat),
                mock.patch.object(
                    self.finalizer.os, "replace", wraps=original_replace
                ) as replace,
            ):
                self.finalizer._write_tailscale_auth_key("tskey-auth-test-value")

            destination = state_dir / "tailscale-auth-key"
            temporary_key = state_dir / ".tailscale-auth-key.tmp"
            replace.assert_called_once_with(temporary_key, destination)
            self.assertEqual(
                destination.read_text(encoding="utf-8"), "tskey-auth-test-value\n"
            )
            self.assertEqual(stat.S_IMODE(state_dir.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE(destination.stat().st_mode), 0o600)
            self.assertFalse(temporary_key.exists())

            with (
                mock.patch.object(self.finalizer, "SYSROOT", sysroot),
                mock.patch.object(pathlib.Path, "lstat", new=root_owned_lstat),
            ):
                with self.assertRaisesRegex(RuntimeError, "handoff already exists"):
                    self.finalizer._write_tailscale_auth_key("replacement")
            self.assertEqual(
                destination.read_text(encoding="utf-8"), "tskey-auth-test-value\n"
            )

    def test_tailscale_handoff_removes_publication_when_directory_fsync_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            sysroot = pathlib.Path(temporary) / "sysroot"
            sysroot.mkdir()
            state_dir = sysroot / "var/lib/soda-install"
            original_lstat = pathlib.Path.lstat
            original_fsync = os.fsync

            def root_owned_lstat(path):
                metadata = original_lstat(path)
                if path != state_dir:
                    return metadata
                return types.SimpleNamespace(
                    st_mode=metadata.st_mode,
                    st_uid=0,
                    st_gid=0,
                )

            def fail_directory_fsync(descriptor):
                if stat.S_ISDIR(os.fstat(descriptor).st_mode):
                    raise OSError("directory fsync failed")
                return original_fsync(descriptor)

            with (
                mock.patch.object(self.finalizer, "SYSROOT", sysroot),
                mock.patch.object(pathlib.Path, "lstat", new=root_owned_lstat),
                mock.patch.object(
                    self.finalizer.os,
                    "fsync",
                    side_effect=fail_directory_fsync,
                ),
            ):
                with self.assertRaisesRegex(OSError, "directory fsync failed"):
                    self.finalizer._write_tailscale_auth_key(
                        "tskey-auth-test-value"
                    )

            self.assertFalse((state_dir / "tailscale-auth-key").exists())
            self.assertFalse((state_dir / ".tailscale-auth-key.tmp").exists())


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("usage: installer_executables_test.py INPUT FINALIZER")
    INPUT_PATH = pathlib.Path(sys.argv[1])
    FINALIZER_PATH = pathlib.Path(sys.argv[2])
    sys.argv = [sys.argv[0]]
    unittest.main()
