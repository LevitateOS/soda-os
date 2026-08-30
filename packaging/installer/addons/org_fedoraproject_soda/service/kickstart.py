from pyanaconda.core.kickstart import KickstartSpecification
from pyanaconda.core.kickstart.addon import AddonData
from pykickstart.errors import KickstartParseError


class SodaIdentityData(AddonData):
    def __init__(self):
        super().__init__()
        self.username = ""
        self.name = ""
        self.email = ""

    def handle_header(self, args, line_number=None):
        if args:
            raise KickstartParseError(
                "Soda identity add-on accepts no header arguments",
                lineno=line_number,
            )

    def handle_line(self, line, line_number=None):
        key, separator, value = line.strip().partition("=")
        if not separator or key not in {"username", "name", "email"}:
            raise KickstartParseError(
                "invalid Soda identity add-on value",
                lineno=line_number,
            )
        setattr(self, key, value)

    def __str__(self):
        return "\n%addon org_fedoraproject_soda\nusername={}\nname={}\nemail={}\n%end\n".format(
            self.username, self.name, self.email
        )


class SodaIdentityKickstartSpecification(KickstartSpecification):
    addons = {"org_fedoraproject_soda": SodaIdentityData}
