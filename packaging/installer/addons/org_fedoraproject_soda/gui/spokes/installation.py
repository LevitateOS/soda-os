import subprocess

from pyanaconda import input_checking
from pyanaconda.core import constants
from pyanaconda.core.constants import PASSWORD_POLICY_USER
from pyanaconda.core.i18n import _
from pyanaconda.core.users import check_username
from pyanaconda.modules.common.constants.services import USERS
from pyanaconda.modules.common.structures.sshkey import SshKeyData
from pyanaconda.modules.common.structures.user import UserData
from pyanaconda.ui.categories.user_settings import UserSettingsCategory
from pyanaconda.ui.gui.helpers import GUISpokeInputCheckHandler
from pyanaconda.ui.gui.spokes import NormalSpoke
from pyanaconda.ui.lib.users import get_user_list, set_user_list

from org_fedoraproject_soda.constants import SODA_INSTALLER

__all__ = ["SodaInstallationSpoke"]

N_ = lambda value: value


def valid_ssh_public_key(value):
    if not value:
        return False
    try:
        result = subprocess.run(
            ["/usr/bin/ssh-keygen", "-l", "-f", "-"],
            input=value + "\n",
            text=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=2,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return False
    return result.returncode == 0


class SodaInstallationSpoke(NormalSpoke, GUISpokeInputCheckHandler):
    builderObjects = ["sodaInstallationWindow"]
    mainWidgetName = "sodaInstallationWindow"
    focusWidgetName = "usernameEntry"
    uiFile = "installation.glade"
    category = UserSettingsCategory
    icon = "avatar-default-symbolic"
    title = N_("_Soda Administrator")

    @staticmethod
    def get_screen_id():
        return "soda-administrator"

    def __init__(self, *args, **kwargs):
        NormalSpoke.__init__(self, *args, **kwargs)
        GUISpokeInputCheckHandler.__init__(self)
        self._installer = SODA_INSTALLER.get_proxy()
        self._users = USERS.get_proxy()
        self._username = None
        self._ssh_public_key = None
        self._tailscale_auth_key = None

    def initialize(self):
        NormalSpoke.initialize(self)
        self.initialize_start()
        self._username = self.builder.get_object("usernameEntry")
        self._password_entry = self.builder.get_object("passwordEntry")
        self._password_confirmation_entry = self.builder.get_object(
            "passwordConfirmationEntry"
        )
        self._ssh_public_key = self.builder.get_object("sshPublicKeyEntry")
        self._tailscale_auth_key = self.builder.get_object(
            "tailscaleAuthKeyEntry"
        )
        self._password_bar = self.builder.get_object("passwordBar")
        self._password_label = self.builder.get_object("passwordLabel")

        self._checker = input_checking.PasswordChecker(
            initial_password_content="",
            initial_password_confirmation_content="",
            policy_name=PASSWORD_POLICY_USER,
        )
        self.checker.secret_type = constants.SecretType.PASSWORD
        self.checker.checks_done.connect(self._checks_done)

        self._username_check = input_checking.UsernameCheck()
        self._empty_check = input_checking.PasswordEmptyCheck()
        self._confirm_check = input_checking.PasswordConfirmationCheck()
        self._validity_check = input_checking.PasswordValidityCheck()
        self._ascii_check = input_checking.PasswordASCIICheck()
        self._validity_check.result.password_score_changed.connect(
            self.set_password_score
        )
        self._validity_check.result.status_text_changed.connect(
            self.set_password_status
        )
        for check in (
            self._username_check,
            self._empty_check,
            self._confirm_check,
            self._validity_check,
            self._ascii_check,
        ):
            self.checker.add_check(check)

        self.password_bar.add_offset_value("low", 2)
        self.password_bar.add_offset_value("medium", 3)
        self.password_bar.add_offset_value("high", 4)
        for entry in (
            self._username,
            self.password_entry,
            self.password_confirmation_entry,
            self._ssh_public_key,
            self._tailscale_auth_key,
        ):
            entry.connect("changed", self._on_input_changed)

        self._rerun_checks()
        self.initialize_done()

    def refresh(self):
        users = get_user_list(self._users)
        if len(users) == 1:
            user = users[0]
            self._username.set_text(user.name)
            if user.password and not user.is_crypted:
                self.password_entry.set_placeholder_text(
                    "Administrator password already provided"
                )
                self.password_confirmation_entry.set_placeholder_text(
                    "Administrator password already provided"
                )

        keys = SshKeyData.from_structure_list(self._users.SshKeys)
        if len(keys) == 1:
            self._ssh_public_key.set_text(keys[0].key)

        if self._installer.HasTailscaleAuthKey:
            self._tailscale_auth_key.set_placeholder_text(
                "One-use key already provided"
            )
        self._rerun_checks()

    def apply(self):
        if not self.can_go_back:
            return

        username = self._username.get_text()
        users = get_user_list(self._users)
        if len(users) == 1 and users[0].name == username:
            user = users[0]
        else:
            user = UserData()

        user.name = username
        user.gecos = username
        if self.password:
            user.password = self.password
            user.is_crypted = False
        user.lock = False
        user.set_admin_priviledges(True)
        set_user_list(self._users, [user])

        key = SshKeyData()
        key.username = username
        key.key = self._ssh_public_key.get_text().strip()
        self._users.SshKeys = SshKeyData.to_structure_list([key])

        tailscale_auth_key = self._tailscale_auth_key.get_text().strip()
        if tailscale_auth_key:
            self._installer.SetTailscaleAuthKey(tailscale_auth_key)

        self.password_entry.set_text("")
        self.password_confirmation_entry.set_text("")
        self.password_entry.set_placeholder_text(
            "Administrator password already provided"
        )
        self.password_confirmation_entry.set_placeholder_text(
            "Administrator password already provided"
        )
        self._tailscale_auth_key.set_text("")
        self._tailscale_auth_key.set_placeholder_text(
            "One-use key already provided"
        )
        self._rerun_checks()

    def _on_input_changed(self, _entry):
        self._rerun_checks()

    def _rerun_checks(self):
        preserve_password = (
            not self.password
            and not self.password_confirmation
            and self._has_stored_plaintext_password()
        )
        for check in (
            self._empty_check,
            self._confirm_check,
            self._validity_check,
            self._ascii_check,
        ):
            check.skip = preserve_password

        self.checker.username = self._username.get_text()
        if self.checker.password.content != self.password:
            self.checker.password.content = self.password
        if self.checker.password_confirmation.content != self.password_confirmation:
            self.checker.password_confirmation.content = self.password_confirmation
        self.checker.run_checks()

    def _has_stored_plaintext_password(self):
        users = get_user_list(self._users)
        return (
            len(users) == 1
            and users[0].name == self._username.get_text()
            and bool(users[0].password)
            and not users[0].is_crypted
        )

    def _required_field_error(self):
        if not valid_ssh_public_key(self._ssh_public_key.get_text().strip()):
            return "A valid OpenSSH public key is required"
        if (
            not self._tailscale_auth_key.get_text().strip()
            and not self._installer.HasTailscaleAuthKey
        ):
            return "A one-use Tailscale auth key is required"
        return ""

    def _checks_done(self, checker_error):
        extra_error = self._required_field_error()
        error = checker_error or extra_error
        unwaivable = (
            not self._username_check.result.success
            or not self._empty_check.result.success
            or not self._confirm_check.result.success
            or bool(extra_error)
        )

        self.waive_clicks = 0
        if not error:
            self.clear_info()
        elif self.checker.policy.is_strict or unwaivable:
            self.show_warning_message(error)
        else:
            self.show_warning_message(
                _(constants.PASSWORD_ERROR_CONCATENATION).format(
                    error,
                    _(constants.PASSWORD_DONE_TWICE),
                )
            )

        self.can_go_back = False
        self.needs_waiver = False
        if unwaivable:
            return
        if self.checker.success:
            self.can_go_back = True
            return
        if self.checker.policy.is_strict and not self._validity_check.result.success:
            return
        self.can_go_back = True
        self.needs_waiver = True

    def on_back_clicked(self, button):
        if self.try_to_go_back():
            NormalSpoke.on_back_clicked(self, button)

    @property
    def completed(self):
        users = get_user_list(self._users)
        if len(users) != 1:
            return False
        user = users[0]
        valid_username, _ = check_username(user.name)
        if (
            not valid_username
            or not user.has_admin_priviledges()
            or not user.password
            or user.is_crypted
        ):
            return False
        keys = [
            key for key in SshKeyData.from_structure_list(self._users.SshKeys)
            if key.username == user.name and valid_ssh_public_key(key.key.strip())
        ]
        return len(keys) == 1 and self._installer.HasTailscaleAuthKey

    @property
    def mandatory(self):
        return True

    @property
    def status(self):
        users = get_user_list(self._users)
        if self.completed:
            return "Administrator and Tailnet enrollment ready"
        if len(users) == 1 and users[0].name:
            return f"Complete setup for administrator {users[0].name}"
        return "Administrator and Tailnet enrollment required"
