package server

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

//go:embed templates/*.html static/*
var content embed.FS

const sessionCookie = "soda_session"

type Server struct {
	templates *template.Template
	assets    http.Handler
	api       soda.API
	auth      auth.Authenticator
	sessions  *sessionStore
}

type sessionStore struct {
	mu      sync.RWMutex
	byToken map[string]soda.Person
}

type pageData struct {
	Title       string
	Version     string
	User        soda.Person
	People      []soda.Person
	Projects    []soda.Project
	Project     *soda.Project
	Worktrees   []soda.Worktree
	Jobs        []soda.ProvisioningJob
	Toolchain   *soda.ToolchainInstallation
	DeployKey   string
	PersonNames map[string]string
	Error       string
	Admin       bool
}

type userContextKey struct{}

func New(api soda.API, authenticator auth.Authenticator) (*Server, error) {
	templates, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, err
	}
	assetsFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		templates: templates,
		assets:    http.FileServer(http.FS(assetsFS)),
		api:       api,
		auth:      authenticator,
		sessions:  &sessionStore{byToken: make(map[string]soda.Person)},
	}, nil
}

func (s *Server) Handler() http.Handler {
	public := http.NewServeMux()
	public.Handle("GET /static/", http.StripPrefix("/static/", s.assets))
	public.HandleFunc("GET /healthz", health)
	public.HandleFunc("GET /login", s.loginPage)
	public.HandleFunc("POST /login", s.login)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /", s.home)
	protected.HandleFunc("POST /logout", s.logout)
	protected.HandleFunc("GET /people", s.people)
	protected.HandleFunc("POST /people", s.createPerson)
	protected.HandleFunc("GET /projects", s.projects)
	protected.HandleFunc("POST /projects", s.createProject)
	protected.HandleFunc("GET /projects/{project_id}", s.project)
	protected.HandleFunc("POST /projects/{project_id}/collaborators", s.addCollaborator)
	protected.HandleFunc("POST /projects/{project_id}/worktrees", s.createWorktree)
	protected.HandleFunc("POST /projects/{project_id}/provisioning", s.retryProvisioning)
	protected.HandleFunc("GET /profiles", s.profiles)

	public.Handle("/", s.requireSession(protected))
	return securityHeaders(public)
}

func (s *Server) ListenAndServeTLS(address, certFile, keyFile string) error {
	return http.ListenAndServeTLS(address, certFile, keyFile, s.Handler())
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) loginPage(w http.ResponseWriter, _ *http.Request) {
	s.render(w, http.StatusOK, "login.html", pageData{Title: "Sign in · Soda OS", Version: "0.1.0"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid login", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	if err := s.auth.Authenticate(username, r.FormValue("password")); err != nil {
		s.render(w, http.StatusUnauthorized, "login.html", pageData{Title: "Sign in · Soda OS", Version: "0.1.0", Error: "Invalid username or password."})
		return
	}
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "Soda service unavailable", http.StatusBadGateway)
		return
	}
	var person *soda.Person
	for i := range people {
		if people[i].Username == username {
			person = &people[i]
			break
		}
	}
	if person == nil {
		s.render(w, http.StatusForbidden, "login.html", pageData{Title: "Sign in · Soda OS", Version: "0.1.0", Error: "This Linux account is not registered with Soda OS."})
		return
	}
	token, err := s.sessions.create(*person)
	if err != nil {
		http.Error(w, "create session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.remove(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load projects", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "index.html", pageData{Title: "Soda OS", Version: "0.1.0", User: user, Projects: projects})
}

func (s *Server) people(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "people.html", pageData{Title: "People · Soda OS", Version: "0.1.0", User: currentUser(r), People: people})
}

func (s *Server) createPerson(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid person", http.StatusBadRequest)
		return
	}
	_, err := s.api.CreatePerson(r.Context(), soda.CreatePersonRequest{
		Username: r.FormValue("username"), DisplayName: r.FormValue("display_name"),
		Email: r.FormValue("email"), Role: soda.Role(r.FormValue("role")),
		SSHPublicKey: r.FormValue("ssh_public_key"), Password: r.FormValue("password"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/people")
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load projects", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "projects.html", pageData{Title: "Projects · Soda OS", Version: "0.1.0", User: user, Projects: projects})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid project", http.StatusBadRequest)
		return
	}
	source := soda.ProjectSource{Kind: "empty"}
	if remote := strings.TrimSpace(r.FormValue("remote_url")); remote != "" {
		source = soda.ProjectSource{Kind: "git", RemoteURL: remote}
	}
	project, err := s.api.CreateProject(r.Context(), soda.CreateProjectRequest{
		Slug: r.FormValue("slug"), Name: r.FormValue("name"), Profile: r.FormValue("profile"), Source: source,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/projects/"+project.ID)
}

func (s *Server) project(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	worktrees, err := s.api.Worktrees(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load worktrees", http.StatusBadGateway)
		return
	}
	jobs, err := s.api.Jobs(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load provisioning jobs", http.StatusBadGateway)
		return
	}
	installation, err := s.api.Toolchain(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load toolchain", http.StatusBadGateway)
		return
	}
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return
	}
	data := pageData{Title: "Project · Soda OS", Version: "0.1.0", User: user, Project: &project,
		People: people, Worktrees: worktrees, Jobs: jobs, Toolchain: installation, PersonNames: personNames(people)}
	if user.Role == soda.RoleAdmin {
		key, err := s.api.DeployKey(r.Context(), project.ID)
		if err != nil {
			http.Error(w, "load deploy key", http.StatusBadGateway)
			return
		}
		data.DeployKey = key.PublicKey
	}
	s.render(w, http.StatusOK, "project.html", data)
}

func (s *Server) addCollaborator(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid collaborator", http.StatusBadRequest)
		return
	}
	projectID := r.PathValue("project_id")
	if _, err := s.api.AddCollaborator(r.Context(), projectID, r.FormValue("person_id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/projects/"+projectID)
}

func (s *Server) createWorktree(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid worktree", http.StatusBadRequest)
		return
	}
	projectID := r.PathValue("project_id")
	if _, err := s.api.CreateWorktree(r.Context(), projectID, r.FormValue("person_id"), r.FormValue("name"), r.FormValue("base_ref")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/projects/"+projectID)
}

func (s *Server) retryProvisioning(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	projectID := r.PathValue("project_id")
	if _, err := s.api.RetryProvisioning(r.Context(), projectID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/projects/"+projectID)
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load profiles", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "profiles.html", pageData{Title: "Toolchains · Soda OS", Version: "0.1.0", User: user, Projects: projects})
}

func (s *Server) visibleProjects(ctx context.Context, user soda.Person) ([]soda.Project, error) {
	if user.Role == soda.RoleAdmin {
		return s.api.Projects(ctx)
	}
	return s.api.ProjectsForPerson(ctx, user.ID)
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

func personNames(people []soda.Person) map[string]string {
	names := make(map[string]string, len(people))
	for _, person := range people {
		names[person.ID] = person.DisplayName
	}
	return names
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data pageData) {
	data.Admin = data.User.Role == soda.RoleAdmin
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func redirect(w http.ResponseWriter, r *http.Request, location string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
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
