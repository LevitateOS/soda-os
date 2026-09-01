from pyanaconda.core.configuration.anaconda import conf
from pyanaconda.core.dbus import DBus
from pyanaconda.core.signal import Signal
from pyanaconda.modules.common.base import KickstartService
from pyanaconda.modules.common.containers import TaskContainer

from org_fedoraproject_soda.constants import SODA_INSTALLER
from org_fedoraproject_soda.service.installation import ProvisionSodaInstallationTask
from org_fedoraproject_soda.service.interface import SodaInstallerInterface
from org_fedoraproject_soda.service.kickstart import SodaInstallerKickstartSpecification


class SodaInstaller(KickstartService):
    def __init__(self):
        super().__init__()
        self._tailscale_auth_key = ""
        self.configuration_changed = Signal()

    def publish(self):
        TaskContainer.set_namespace(SODA_INSTALLER.namespace)
        DBus.publish_object(SODA_INSTALLER.object_path, SodaInstallerInterface(self))
        DBus.register_service(SODA_INSTALLER.service_name)

    @property
    def kickstart_specification(self):
        return SodaInstallerKickstartSpecification

    def process_kickstart(self, data):
        self.set_tailscale_auth_key(
            data.addons.org_fedoraproject_soda.tailscale_auth_key
        )

    def setup_kickstart(self, data):
        # The one-use key must not be copied into Anaconda's retained output
        # Kickstart. The add-on's input parser is intentionally one-way.
        data.addons.org_fedoraproject_soda.tailscale_auth_key = ""

    @property
    def has_tailscale_auth_key(self):
        return bool(self._tailscale_auth_key)

    def set_tailscale_auth_key(self, auth_key):
        self._tailscale_auth_key = auth_key.strip()
        self.configuration_changed.emit()

    def install_with_tasks(self):
        task = ProvisionSodaInstallationTask(
            conf.target.system_root,
            conf.target.physical_root,
            self._tailscale_auth_key,
        )
        self._tailscale_auth_key = ""
        self.configuration_changed.emit()
        return [task]
