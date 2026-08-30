from dasbus.server.interface import dbus_interface
from dasbus.server.property import emits_properties_changed
from dasbus.typing import Str
from pyanaconda.modules.common.base import KickstartModuleInterface

from org_fedoraproject_soda.constants import SODA_IDENTITY


@dbus_interface(SODA_IDENTITY.interface_name)
class SodaIdentityInterface(KickstartModuleInterface):
    def connect_signals(self):
        super().connect_signals()
        self.watch_property("Username", self.implementation.identity_changed)
        self.watch_property("FullName", self.implementation.identity_changed)
        self.watch_property("Email", self.implementation.identity_changed)

    @property
    def Username(self) -> Str:
        return self.implementation.username

    @property
    def FullName(self) -> Str:
        return self.implementation.name

    @property
    def Email(self) -> Str:
        return self.implementation.email

    @emits_properties_changed
    def SetIdentity(self, username: Str, name: Str, email: Str):
        self.implementation.set_identity(username, name, email)
