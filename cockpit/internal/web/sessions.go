package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

type sessionStore struct {
	mu      sync.RWMutex
	byToken map[string]daemonclient.Person
}

type userContextKey struct{}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		person, ok := s.sessions.get(cookie.Value)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, person)))
	})
}

func currentUser(r *http.Request) daemonclient.Person {
	return r.Context().Value(userContextKey{}).(daemonclient.Person)
}

func (s *sessionStore) create(person daemonclient.Person) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	s.mu.Lock()
	s.byToken[token] = person
	s.mu.Unlock()
	return token, nil
}

func (s *sessionStore) get(token string) (daemonclient.Person, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	person, ok := s.byToken[token]
	return person, ok
}

func (s *sessionStore) remove(token string) {
	s.mu.Lock()
	delete(s.byToken, token)
	s.mu.Unlock()
}
