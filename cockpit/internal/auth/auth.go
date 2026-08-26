package auth

type Authenticator interface {
	Authenticate(username, password string) error
}

type PAM struct{}

func NewPAM() PAM {
	return PAM{}
}
