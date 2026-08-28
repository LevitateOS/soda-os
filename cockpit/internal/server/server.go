package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
	"github.com/LevitateOS/soda-os/internal/version"
)

//go:embed templates/*.html static/*
var content embed.FS

const sessionCookie = "soda_session"

type Server struct {
	templates  *template.Template
	assets     http.Handler
	accounts   accountPort
	projectAPI projectPort
	host       hostPort
	updates    updatePort
	auth       auth.Authenticator
	sessions   *sessionStore
}

// These ports stay with the HTTP consumers. The daemon client is only one
// implementation; each page receives the smallest capability it needs.
type accountPort interface {
	People(context.Context) ([]soda.Person, error)
	CreatePerson(context.Context, soda.CreatePersonRequest) error
	SSHDeviceKeys(context.Context, string) ([]soda.SSHDeviceKey, error)
	CreateSSHDeviceKey(context.Context, string, string, string, string) error
	RevokeSSHDeviceKey(context.Context, string, string) error
}

type projectPort interface {
	Projects(context.Context) ([]soda.Project, error)
	ProjectsForPerson(context.Context, string) ([]soda.Project, error)
	CreateProject(context.Context, soda.CreateProjectRequest) (soda.Project, error)
	Members(context.Context, string) ([]soda.Person, error)
	AddCollaborator(context.Context, soda.AddCollaboratorCommand) error
	Worktrees(context.Context, string) ([]soda.Worktree, error)
	Jobs(context.Context, string) ([]soda.ProvisioningJob, error)
	RetryProvisioning(context.Context, string) error
	Toolchain(context.Context, string) (*soda.ToolchainInstallation, error)
	DeployKey(context.Context, string) (string, error)
}

type hostPort interface {
	HostStatus(context.Context) (soda.HostStatus, error)
}

type updatePort interface {
	OSUpdateStatus(context.Context) (soda.OSUpdateStatus, error)
	CheckOSUpdate(context.Context) (soda.OSRelease, error)
	StageOSUpdate(context.Context, string) (soda.OSUpdateStatus, error)
	ActivateOSUpdate(context.Context) error
}

type sessionStore struct {
	mu      sync.RWMutex
	byToken map[string]soda.Person
}

type pageIdentity struct {
	Title string
	User  soda.Person
}

func (view pageIdentity) Admin() bool { return view.User.Role == soda.RoleAdmin }

type loginView struct {
	pageIdentity
	Error                  string
	PasswordChangeRequired bool
	Username               string
}

type projectListView struct {
	ProjectCards  []projectCardView
	ProjectsError string
}

type homeView struct {
	pageIdentity
	projectListView
	Host      *soda.HostStatus
	HostError string
}

type accountView struct {
	pageIdentity
	DeviceKeys []soda.SSHDeviceKey
	Message    string
	Error      string
}

type peopleView struct {
	pageIdentity
	People  []soda.Person
	Message string
	Error   string
}

type projectsView struct {
	pageIdentity
	projectListView
	People []soda.Person
	Error  string
}

type projectStateView struct {
	Label  string
	Class  string
	Ready  bool
	Active bool
}

type provisioningView struct {
	Project   soda.Project
	Admin     bool
	State     projectStateView
	Jobs      []soda.ProvisioningJob
	Toolchain *soda.ToolchainInstallation
	Error     string
}

type collaborationView struct {
	Project   soda.Project
	User      soda.Person
	Admin     bool
	Members   []memberWorkspaceView
	Available []soda.Person
	Ready     bool
	Message   string
	Error     string
}

type connectView struct {
	Project     soda.Project
	User        soda.Person
	State       projectStateView
	DeviceKeys  []soda.SSHDeviceKey
	SelectedKey *soda.SSHDeviceKey
	Workspace   *soda.Worktree
	SSHConfig   string
	SSHCommand  string
	Error       string
}

type projectView struct {
	pageIdentity
	Project       soda.Project
	State         projectStateView
	Connection    connectView
	Provisioning  provisioningView
	Collaboration collaborationView
	DeployKey     string
}

type profilesView struct {
	pageIdentity
	Projects []soda.Project
}

type osUpdatePageView struct {
	pageIdentity
	OSUpdate  *soda.OSUpdateStatus
	OSRelease *soda.OSRelease
	Message   string
	Error     string
}

type projectCardView struct {
	Project    soda.Project
	State      string
	StateClass string
}

type memberWorkspaceView struct {
	Person    soda.Person
	Workspace *soda.Worktree
}

type osUpdateView struct {
	status  int
	message string
	error   string
	release *soda.OSRelease
	value   *soda.OSUpdateStatus
}

type userContextKey struct{}

func New(accounts accountPort, projects projectPort, host hostPort, updates updatePort, authenticator auth.Authenticator) (*Server, error) {
	templates, err := template.New("root").Funcs(template.FuncMap{
		"bytes":    humanBytes,
		"duration": humanDuration,
		"keyType":  sshKeyType,
		"remote":   projectRemote,
		"time":     humanTime,
		"version":  func() string { return version.Version },
	}).ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, err
	}
	assetsFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		templates:  templates,
		assets:     http.FileServer(http.FS(assetsFS)),
		accounts:   accounts,
		projectAPI: projects,
		host:       host,
		updates:    updates,
		auth:       authenticator,
		sessions:   &sessionStore{byToken: make(map[string]soda.Person)},
	}, nil
}

func (s *Server) Handler() http.Handler {
	public := http.NewServeMux()
	public.Handle("GET /static/", http.StripPrefix("/static/", s.assets))
	public.HandleFunc("GET /healthz", health)
	public.HandleFunc("GET /login", s.loginPage)
	public.HandleFunc("POST /login", s.login)
	public.HandleFunc("POST /activate-password", s.activatePassword)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /", s.home)
	protected.HandleFunc("POST /logout", s.logout)
	protected.HandleFunc("GET /account", s.account)
	protected.HandleFunc("POST /account/password", s.changePassword)
	protected.HandleFunc("POST /account/ssh-keys", s.createSSHDeviceKey)
	protected.HandleFunc("POST /account/ssh-keys/{key_id}/revoke", s.revokeSSHDeviceKey)
	protected.HandleFunc("GET /team", s.people)
	protected.HandleFunc("POST /team", s.createPerson)
	protected.HandleFunc("GET /projects", s.projects)
	protected.HandleFunc("POST /projects", s.createProject)
	protected.HandleFunc("GET /projects/{project_id}", s.project)
	protected.HandleFunc("GET /projects/{project_id}/connect", s.connectFragment)
	protected.HandleFunc("GET /projects/{project_id}/ssh-config", s.sshConfig)
	protected.HandleFunc("GET /projects/{project_id}/provisioning", s.provisioning)
	protected.HandleFunc("POST /projects/{project_id}/members", s.addCollaborator)
	protected.HandleFunc("POST /projects/{project_id}/provisioning", s.retryProvisioning)
	protected.HandleFunc("GET /profiles", s.profiles)
	protected.HandleFunc("GET /os-update", s.osUpdate)
	protected.HandleFunc("POST /os-update/check", s.checkOSUpdate)
	protected.HandleFunc("POST /os-update/stage", s.stageOSUpdate)
	protected.HandleFunc("POST /os-update/activate", s.activateOSUpdate)

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
