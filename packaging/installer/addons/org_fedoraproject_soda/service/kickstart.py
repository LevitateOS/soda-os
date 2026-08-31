from pyanaconda.core.kickstart import KickstartSpecification
from pyanaconda.core.kickstart.addon import AddonData
from pykickstart.errors import KickstartParseError


class SodaInstallerData(AddonData):
    def __init__(self):
        super().__init__()
        self.tailscale_auth_key = ""

    def handle_header(self, args, line_number=None):
        if args:
            raise KickstartParseError(
                "Soda installer add-on accepts no header arguments",
                lineno=line_number,
            )

    def handle_line(self, line, line_number=None):
        key, separator, value = line.strip().partition("=")
        if not separator or key != "tailscale_auth_key":
            raise KickstartParseError(
                "invalid Soda installer add-on value",
                lineno=line_number,
            )
        setattr(self, key, value)

    def __str__(self):
        # Never reproduce the one-use enrollment key in output Kickstart.
        return ""


class SodaInstallerKickstartSpecification(KickstartSpecification):
    addons = {"org_fedoraproject_soda": SodaInstallerData}
