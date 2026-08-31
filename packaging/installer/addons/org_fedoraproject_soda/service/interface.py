from dasbus.server.interface import dbus_interface
from dasbus.server.property import emits_properties_changed
from dasbus.typing import Bool, Str
from pyanaconda.modules.common.base import KickstartModuleInterface

from org_fedoraproject_soda.constants import SODA_INSTALLER


@dbus_interface(SODA_INSTALLER.interface_name)
class SodaInstallerInterface(KickstartModuleInterface):
    def connect_signals(self):
        super().connect_signals()
        self.watch_property(
            "HasTailscaleAuthKey",
            self.implementation.configuration_changed,
        )

    @property
    def HasTailscaleAuthKey(self) -> Bool:
        return self.implementation.has_tailscale_auth_key

    @emits_properties_changed
    def SetTailscaleAuthKey(self, auth_key: Str):
        self.implementation.set_tailscale_auth_key(auth_key)
