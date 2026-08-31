from dasbus.identifier import DBusServiceIdentifier
from pyanaconda.core.dbus import DBus
from pyanaconda.modules.common.constants.namespaces import ADDONS_NAMESPACE

SODA_INSTALLER = DBusServiceIdentifier(
    namespace=(*ADDONS_NAMESPACE, "SodaInstaller"),
    message_bus=DBus,
)
