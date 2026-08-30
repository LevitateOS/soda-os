from pyanaconda.modules.common import init

init()

from org_fedoraproject_soda.service.identity import SodaIdentity

SodaIdentity().run()
