package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

func (s *Server) visibleProjects(ctx context.Context, user soda.Person) ([]soda.Project, error) {
	if user.Role == soda.RoleAdmin {
		return s.projectAPI.Projects(ctx)
	}
	return s.projectAPI.ProjectsForPerson(ctx, user.ID)
}

func (s *Server) provisioningState(ctx context.Context, projectID string) ([]soda.ProvisioningJob, *soda.ToolchainInstallation, error) {
	jobs, err := s.projectAPI.Jobs(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	installation, err := s.projectAPI.Toolchain(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	return jobs, installation, nil
}

func provisioningActive(jobs []soda.ProvisioningJob) bool {
	for _, job := range jobs {
		if job.State == "installing" {
			return true
		}
	}
	return false
}

func projectState(jobs []soda.ProvisioningJob) (string, string) {
	if len(jobs) == 0 {
		return "Preparing", "preparing"
	}
	switch jobs[0].State {
	case "ready":
		return "Ready", "ready"
	case "failed":
		return "Needs attention", "failed"
	default:
		return "Preparing", "preparing"
	}
}

func (s *Server) projectCards(ctx context.Context, projects []soda.Project) ([]projectCardView, error) {
	cards := make([]projectCardView, 0, len(projects))
	for _, project := range projects {
		jobs, loadErr := s.projectAPI.Jobs(ctx, project.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		state, stateClass := projectState(jobs)
		cards = append(cards, projectCardView{
			Project: project, State: state, StateClass: stateClass,
		})
	}
	return cards, nil
}

func peopleWithoutMembers(people, projectMembers []soda.Person) []soda.Person {
	members := make(map[string]struct{}, len(projectMembers))
	for _, person := range projectMembers {
		members[person.ID] = struct{}{}
	}
	available := make([]soda.Person, 0, len(people))
	for _, person := range people {
		if _, exists := members[person.ID]; !exists {
			available = append(available, person)
		}
	}
	return available
}

func memberWorkspaceViews(members []soda.Person, worktrees []soda.Worktree) []memberWorkspaceView {
	workspaceByPerson := make(map[string]soda.Worktree, len(worktrees))
	for _, worktree := range worktrees {
		workspaceByPerson[worktree.PersonID] = worktree
	}
	views := make([]memberWorkspaceView, 0, len(members))
	for _, person := range members {
		view := memberWorkspaceView{Person: person}
		if workspace, exists := workspaceByPerson[person.ID]; exists {
			workspaceCopy := workspace
			view.Workspace = &workspaceCopy
		}
		views = append(views, view)
	}
	return views
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

func personalizedSSHConfig(project soda.Project, key soda.SSHDeviceKey) string {
	return fmt.Sprintf("Host soda-%s\n    HostName soda.local\n    User %s\n    IdentityFile %s\n    IdentitiesOnly yes\n", project.Slug, project.UnixUser, sshConfigValue(key.IdentityFileHint))
}

func personalizedSSHCommand(project soda.Project, key soda.SSHDeviceKey) string {
	return fmt.Sprintf("ssh -i %s %s@soda.local", shellPath(key.IdentityFileHint), project.UnixUser)
}

func sshConfigValue(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
	return `"` + escaped + `"`
}

func shellPath(value string) string {
	if value == "~" {
		return `"$HOME"`
	}
	if strings.HasPrefix(value, "~/") {
		return `"$HOME"/` + shellQuote(strings.TrimPrefix(value, "~/"))
	}
	return shellQuote(value)
}

func shellQuote(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `'"'"'`) + `'`
}

func (s *Server) visibleProject(ctx context.Context, user soda.Person, projectID string) (soda.Project, bool, error) {
	projects, err := s.visibleProjects(ctx, user)
	if err != nil {
		return soda.Project{}, false, err
	}
	for _, project := range projects {
		if project.ID == projectID {
			return project, true, nil
		}
	}
	return soda.Project{}, false, nil
}

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

func currentUser(r *http.Request) soda.Person {
	return r.Context().Value(userContextKey{}).(soda.Person)
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if currentUser(r).Role != soda.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func redirect(w http.ResponseWriter, r *http.Request, location string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func humanBytes(value uint64) string {
	const unit = uint64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor := unit
	exponent := 0
	for remaining := value / unit; remaining >= unit && exponent < 5; remaining /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func humanDuration(seconds uint64) string {
	duration := time.Duration(seconds) * time.Second
	if duration >= 24*time.Hour {
		return fmt.Sprintf("%dd %dh", int(duration/(24*time.Hour)), int(duration/time.Hour)%24)
	}
	return duration.Round(time.Minute).String()
}

func humanTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Format("2006-01-02 15:04:05")
}

func sshKeyType(key soda.SSHDeviceKey) string {
	value, _, _ := strings.Cut(key.PublicKey, " ")
	return value
}

func projectRemote(project soda.Project) string {
	if source, ok := project.Source.(soda.GitProjectSource); ok {
		return source.RemoteURL
	}
	return ""
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (s *sessionStore) create(person soda.Person) (string, error) {
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

func (s *sessionStore) get(token string) (soda.Person, bool) {
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
