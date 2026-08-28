package web

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

type loginView struct {
	pageIdentity
	Error                  string
	PasswordChangeRequired bool
	Username               string
}

type accountView struct {
	pageIdentity
	DeviceKeys []daemonclient.SSHDeviceKey
	Message    string
	Error      string
}

type peopleView struct {
	pageIdentity
	People  []daemonclient.Person
	Message string
	Error   string
}

func validatePassword(password string) error {
	if strings.ContainsAny(password, "\r\n\x00") {
		return fmt.Errorf("password cannot contain line breaks")
	}
	if utf8.RuneCountInString(password) < 6 {
		return fmt.Errorf("password must be at least six characters")
	}
	return nil
}

func (s *Server) loginPage(w http.ResponseWriter, _ *http.Request) {
	s.render(w, http.StatusOK, "login.html", loginView{pageIdentity: pageIdentity{Title: "Sign in · Soda OS"}})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid login", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	result, err := s.auth.Authenticate(username, r.FormValue("password"))
	if err != nil {
		s.render(w, http.StatusUnauthorized, "login.html", loginView{pageIdentity: pageIdentity{Title: "Sign in · Soda OS"}, Error: "Invalid username or password."})
		return
	}
	if result == auth.PasswordChangeRequired {
		s.render(w, http.StatusOK, "login.html", loginView{pageIdentity: pageIdentity{Title: "Activate account · Soda OS"}, PasswordChangeRequired: true, Username: username})
		return
	}
	s.finishLogin(w, r, username, "/")
}

func (s *Server) activatePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid password change", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	current := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	if newPassword != r.FormValue("confirm_password") {
		s.render(w, http.StatusUnprocessableEntity, "login.html", loginView{pageIdentity: pageIdentity{Title: "Activate account · Soda OS"}, PasswordChangeRequired: true, Username: username, Error: "New passwords do not match."})
		return
	}
	if err := validatePassword(newPassword); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "login.html", loginView{pageIdentity: pageIdentity{Title: "Activate account · Soda OS"}, PasswordChangeRequired: true, Username: username, Error: err.Error()})
		return
	}
	if err := s.auth.ChangePassword(username, current, newPassword); err != nil {
		s.render(w, http.StatusUnauthorized, "login.html", loginView{pageIdentity: pageIdentity{Title: "Activate account · Soda OS"}, PasswordChangeRequired: true, Username: username, Error: "The current password was invalid or the password could not be changed."})
		return
	}
	s.finishLogin(w, r, username, "/account")
}

func (s *Server) finishLogin(w http.ResponseWriter, r *http.Request, username, destination string) {
	people, err := s.accounts.People(r.Context())
	if err != nil {
		http.Error(w, "Soda service unavailable", http.StatusBadGateway)
		return
	}
	var person *daemonclient.Person
	for i := range people {
		if people[i].Username == username {
			person = &people[i]
			break
		}
	}
	if person == nil {
		s.render(w, http.StatusForbidden, "login.html", loginView{pageIdentity: pageIdentity{Title: "Sign in · Soda OS"}, Error: "This Linux account is not registered with Soda OS."})
		return
	}
	token, err := s.sessions.create(*person)
	if err != nil {
		http.Error(w, "create session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.remove(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	keys, err := s.accounts.SSHDeviceKeys(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "load SSH devices", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "account.html", accountView{pageIdentity: pageIdentity{Title: "My account · Soda OS", User: user}, DeviceKeys: keys})
}

func (s *Server) createSSHDeviceKey(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid SSH device", http.StatusBadRequest)
		return
	}
	user := currentUser(r)
	err := s.accounts.CreateSSHDeviceKey(r.Context(), user.ID, r.FormValue("label"), r.FormValue("public_key"), r.FormValue("identity_file_hint"))
	if err != nil {
		s.renderSSHKeysResult(w, r, http.StatusUnprocessableEntity, "", err.Error())
		return
	}
	if isHTMX(r) {
		s.renderSSHKeysResult(w, r, http.StatusOK, "SSH device added.", "")
		return
	}
	redirect(w, r, "/account")
}

func (s *Server) revokeSSHDeviceKey(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if err := s.accounts.RevokeSSHDeviceKey(r.Context(), user.ID, r.PathValue("key_id")); err != nil {
		s.renderSSHKeysResult(w, r, http.StatusUnprocessableEntity, "", err.Error())
		return
	}
	if isHTMX(r) {
		s.renderSSHKeysResult(w, r, http.StatusOK, "SSH device revoked. Existing sessions remain connected.", "")
		return
	}
	redirect(w, r, "/account")
}

func (s *Server) renderSSHKeysResult(w http.ResponseWriter, r *http.Request, status int, message, errorMessage string) {
	user := currentUser(r)
	keys, _ := s.accounts.SSHDeviceKeys(r.Context(), user.ID)
	s.render(w, status, "ssh-keys", accountView{pageIdentity: pageIdentity{User: user}, DeviceKeys: keys, Message: message, Error: errorMessage})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid password change", http.StatusBadRequest)
		return
	}
	user := currentUser(r)
	newPassword := r.FormValue("new_password")
	if newPassword != r.FormValue("confirm_password") {
		s.render(w, http.StatusUnprocessableEntity, "password-change", accountView{pageIdentity: pageIdentity{User: user}, Error: "New passwords do not match."})
		return
	}
	if err := validatePassword(newPassword); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "password-change", accountView{pageIdentity: pageIdentity{User: user}, Error: err.Error()})
		return
	}
	if err := s.auth.ChangePassword(user.Username, r.FormValue("current_password"), newPassword); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "password-change", accountView{pageIdentity: pageIdentity{User: user}, Error: "The current password was invalid or the password could not be changed."})
		return
	}
	s.render(w, http.StatusOK, "password-change", accountView{pageIdentity: pageIdentity{User: user}, Message: "Password changed."})
}

func (s *Server) people(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	people, err := s.accounts.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "people.html", peopleView{pageIdentity: pageIdentity{Title: "Team · Soda OS", User: currentUser(r)}, People: people})
}

func (s *Server) createPerson(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid person", http.StatusBadRequest)
		return
	}
	err := s.accounts.CreatePerson(r.Context(), daemonclient.CreatePersonRequest{
		Username: r.FormValue("username"), DisplayName: r.FormValue("display_name"),
		Email: r.FormValue("email"), Role: daemonclient.Role(r.FormValue("role")),
		Password: r.FormValue("password"),
	})
	if err != nil {
		if isHTMX(r) {
			people, _ := s.accounts.People(r.Context())
			s.render(w, http.StatusUnprocessableEntity, "people-management", peopleView{pageIdentity: pageIdentity{User: currentUser(r)}, People: people, Error: err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if isHTMX(r) {
		people, loadErr := s.accounts.People(r.Context())
		if loadErr != nil {
			http.Error(w, "load people", http.StatusBadGateway)
			return
		}
		s.render(w, http.StatusOK, "people-management", peopleView{pageIdentity: pageIdentity{User: currentUser(r)}, People: people, Message: "Team member added."})
		return
	}
	redirect(w, r, "/team")
}
