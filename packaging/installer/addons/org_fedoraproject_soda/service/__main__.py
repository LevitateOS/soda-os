from pyanaconda.modules.common import init

init()

from org_fedoraproject_soda.service.installer import SodaInstaller

SodaInstaller().run()
