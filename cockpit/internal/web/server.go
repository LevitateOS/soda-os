package web

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
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
	address    string
}

type Ports struct {
	Accounts accountPort
	Projects projectPort
	Host     hostPort
	Updates  updatePort
}

type accountPort interface {
	People(context.Context) ([]daemonclient.Person, error)
	CreatePerson(context.Context, daemonclient.CreatePersonRequest) error
	SSHDeviceKeys(context.Context, string) ([]daemonclient.SSHDeviceKey, error)
	CreateSSHDeviceKey(context.Context, string, string, string, string) error
	RevokeSSHDeviceKey(context.Context, string, string) error
}

type projectPort interface {
	Projects(context.Context) ([]daemonclient.Project, error)
	ProjectsForPerson(context.Context, string) ([]daemonclient.Project, error)
	CreateProject(context.Context, daemonclient.CreateProjectRequest) (daemonclient.Project, error)
	Members(context.Context, string) ([]daemonclient.Person, error)
	AddCollaborator(context.Context, daemonclient.AddCollaboratorCommand) error
	Worktrees(context.Context, string) ([]daemonclient.Worktree, error)
	Jobs(context.Context, string) ([]daemonclient.ProvisioningJob, error)
	RetryProvisioning(context.Context, string) error
	Toolchain(context.Context, string) (*daemonclient.ToolchainInstallation, error)
	DeployKey(context.Context, string) (string, error)
}

type hostPort interface {
	HostStatus(context.Context) (daemonclient.HostStatus, error)
}

type updatePort interface {
	OSUpdateStatus(context.Context) (daemonclient.OSUpdateStatus, error)
	CheckOSUpdate(context.Context) (daemonclient.OSRelease, error)
	StageOSUpdate(context.Context, string) (daemonclient.OSUpdateStatus, error)
	ActivateOSUpdate(context.Context) error
}

type pageIdentity struct {
	Title string
	User  daemonclient.Person
}

func (view pageIdentity) Admin() bool { return view.User.Role == daemonclient.RoleAdmin }

func New(ports Ports, authenticator auth.Authenticator, address string) (*Server, error) {
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
		accounts:   ports.Accounts,
		projectAPI: ports.Projects,
		host:       ports.Host,
		updates:    ports.Updates,
		auth:       authenticator,
		sessions:   &sessionStore{byToken: make(map[string]daemonclient.Person)},
		address:    address,
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
