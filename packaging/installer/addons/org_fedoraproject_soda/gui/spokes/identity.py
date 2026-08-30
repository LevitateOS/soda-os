import re

from pyanaconda.modules.common.constants.services import USERS
from pyanaconda.ui.categories.user_settings import UserSettingsCategory
from pyanaconda.ui.gui.spokes import NormalSpoke
from pyanaconda.ui.lib.users import get_user_list

from org_fedoraproject_soda.constants import SODA_IDENTITY

__all__ = ["SodaIdentitySpoke"]

N_ = lambda value: value
EMAIL = re.compile(r"^[^\s@]+@[^\s@]+$")


class SodaIdentitySpoke(NormalSpoke):
    builderObjects = ["sodaIdentityWindow"]
    mainWidgetName = "sodaIdentityWindow"
    focusWidgetName = "emailEntry"
    uiFile = "identity.glade"
    category = UserSettingsCategory
    icon = "avatar-default-symbolic"
    title = N_("_Soda Account")

    @staticmethod
    def get_screen_id():
        return "soda-account"

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._identity = SODA_IDENTITY.get_proxy()
        self._users = USERS.get_proxy()
        self._email = None

    def initialize(self):
        super().initialize()
        self._email = self.builder.get_object("emailEntry")

    def refresh(self):
        self._email.set_text(self._identity.Email)

    def apply(self):
        users = get_user_list(self._users)
        if not users:
            return
        user = users[0]
        self._identity.SetIdentity(user.name, user.gecos, self._email.get_text())

    @property
    def completed(self):
        users = get_user_list(self._users)
        return bool(users and users[0].name and users[0].gecos and EMAIL.match(self._identity.Email))

    @property
    def mandatory(self):
        return True

    @property
    def status(self):
        if self.completed:
            return self._identity.Email
        return "Name and email required"
