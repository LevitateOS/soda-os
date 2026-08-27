package auth

type Result string

const (
	Authenticated          Result = "authenticated"
	PasswordChangeRequired Result = "password_change_required"
)

type Authenticator interface {
	Authenticate(username, password string) (Result, error)
	ChangePassword(username, currentPassword, newPassword string) error
}

type PAM struct{}

func NewPAM() PAM {
	return PAM{}
}
