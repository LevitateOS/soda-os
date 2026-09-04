package workspace

import "github.com/LevitateOS/soda-os/internal/linuxhost"

var (
	_ AccountLookup         = (*linuxhost.Native)(nil)
	_ AccountInventory      = (*linuxhost.Native)(nil)
	_ PasswordReader        = (*linuxhost.Native)(nil)
	_ AuthorizedKeys        = (*linuxhost.Native)(nil)
	_ AccountHomes          = (*linuxhost.Native)(nil)
	_ DeletionHost          = (*linuxhost.Native)(nil)
	_ OutboundKeyGenerator  = Repository{}
	_ RepositoryPublication = Repository{}
)
