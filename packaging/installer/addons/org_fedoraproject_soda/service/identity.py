from pyanaconda.core.configuration.anaconda import conf
from pyanaconda.core.dbus import DBus
from pyanaconda.core.signal import Signal
from pyanaconda.modules.common.base import KickstartService
from pyanaconda.modules.common.containers import TaskContainer

from org_fedoraproject_soda.constants import SODA_IDENTITY
from org_fedoraproject_soda.service.installation import WriteInstallerPersonTask
from org_fedoraproject_soda.service.interface import SodaIdentityInterface
from org_fedoraproject_soda.service.kickstart import SodaIdentityKickstartSpecification


class SodaIdentity(KickstartService):
    def __init__(self):
        super().__init__()
        self._username = ""
        self._name = ""
        self._email = ""
        self.identity_changed = Signal()

    def publish(self):
        TaskContainer.set_namespace(SODA_IDENTITY.namespace)
        DBus.publish_object(SODA_IDENTITY.object_path, SodaIdentityInterface(self))
        DBus.register_service(SODA_IDENTITY.service_name)

    @property
    def kickstart_specification(self):
        return SodaIdentityKickstartSpecification

    def process_kickstart(self, data):
        identity = data.addons.org_fedoraproject_soda
        self._username = identity.username
        self._name = identity.name
        self._email = identity.email

    def setup_kickstart(self, data):
        identity = data.addons.org_fedoraproject_soda
        identity.username = self._username
        identity.name = self._name
        identity.email = self._email

    @property
    def username(self):
        return self._username

    @property
    def name(self):
        return self._name

    @property
    def email(self):
        return self._email

    def set_identity(self, username, name, email):
        self._username = username.strip()
        self._name = name.strip()
        self._email = email.strip()
        self.identity_changed.emit()

    def install_with_tasks(self):
        if not self._username or not self._name or not self._email:
            return []
        return [WriteInstallerPersonTask(
            conf.target.system_root, self._username, self._name, self._email
        )]
