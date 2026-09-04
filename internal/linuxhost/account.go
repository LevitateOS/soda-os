package linuxhost

import "strings"

const AdministratorGroup = "wheel"

// Account is Linux-native account evidence. Product packages remain
// responsible for deciding which account shapes belong to their workflows.
type Account struct {
	Username     string
	UID          int
	GID          int
	PrimaryGroup string
	GECOS        string
	Home         string
	Shell        string
	Groups       map[string]bool
}

func (account Account) HasGroup(group string) bool {
	return account.Groups[group]
}

func (account Account) HasInteractiveShell() bool {
	if account.Shell == "" {
		return false
	}
	base := account.Shell[strings.LastIndex(account.Shell, "/")+1:]
	return base != "false" && base != "nologin"
}

func (account Account) IsAdministrator() bool {
	return account.HasGroup(AdministratorGroup)
}

type Group struct {
	Name    string
	GID     int
	Members map[string]bool
}

type PKExecIdentity struct {
	Username string
	UID      int
}
