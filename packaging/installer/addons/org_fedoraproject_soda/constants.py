from dasbus.identifier import DBusServiceIdentifier
from pyanaconda.core.dbus import DBus
from pyanaconda.modules.common.constants.namespaces import ADDONS_NAMESPACE

SODA_IDENTITY = DBusServiceIdentifier(
    namespace=(*ADDONS_NAMESPACE, "SodaIdentity"),
    message_bus=DBus,
)
