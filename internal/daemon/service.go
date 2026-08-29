package daemon

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/LevitateOS/soda-os/internal/builtingit"
	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/LevitateOS/soda-os/internal/toolchain"
	"github.com/LevitateOS/soda-os/internal/version"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Telemetry interface {
	HostStatus(context.Context) (*sodav2.HostStatus, error)
}

type OSUpdater interface {
	Status(context.Context) (osupdate.Status, error)
	Check(context.Context) (osupdate.Candidate, error)
	Stage(context.Context, string) (osupdate.Status, error)
	Activate(context.Context) error
}

type BuiltInGit interface {
	EnsurePerson(context.Context, domain.Person, builtingit.PersonKind) (builtingit.User, error)
	EnsureKey(context.Context, domain.Person, domain.SSHDeviceKey) (builtingit.Key, error)
	DeleteKey(context.Context, string, int64) error
	EnsureRepository(context.Context, domain.Project, []domain.Person, string) (builtingit.Repository, error)
}

type Service struct {
	sodav2.UnimplementedSodaServiceServer
	store        *store.Store
	host         host.Operations
	toolchains   toolchain.Installer
	telemetry    Telemetry
	osUpdates    OSUpdater
	builtInGit   BuiltInGit
	projectsRoot string
	logger       *slog.Logger
	provisioning provisioningRuntime
}

type provisioningRuntime struct {
	background context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	timeout    time.Duration
}

type Options struct {
	Store               *store.Store
	Host                host.Operations
	Toolchains          toolchain.Installer
	Telemetry           Telemetry
	OSUpdates           OSUpdater
	BuiltInGit          BuiltInGit
	ProjectsRoot        string
	Logger              *slog.Logger
	ProvisioningTimeout time.Duration
}

const defaultProvisioningTimeout = 30 * time.Minute

func New(options Options) *Service {
	background, cancel := context.WithCancel(context.Background())
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	provisioningTimeout := options.ProvisioningTimeout
	if provisioningTimeout <= 0 {
		provisioningTimeout = defaultProvisioningTimeout
	}
	return &Service{store: options.Store, host: options.Host, toolchains: options.Toolchains, telemetry: options.Telemetry, osUpdates: options.OSUpdates, builtInGit: options.BuiltInGit, projectsRoot: options.ProjectsRoot, logger: logger, provisioning: provisioningRuntime{background: background, cancel: cancel, timeout: provisioningTimeout}}
}
func (s *Service) Close() { s.provisioning.cancel(); s.provisioning.wg.Wait() }

func (s *Service) Health(_ context.Context, _ *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ok", Service: "sodad", Version: version.Version}, nil
}

func (s *Service) GetOSUpdateStatus(ctx context.Context, _ *sodav2.GetOSUpdateStatusRequest) (*sodav2.GetOSUpdateStatusResponse, error) {
	if s.osUpdates == nil {
		return nil, status.Error(codes.Unavailable, "OS update service is unavailable")
	}
	value, err := s.osUpdates.Status(ctx)
	if err != nil {
		return nil, osUpdateRPCError(err)
	}
	return &sodav2.GetOSUpdateStatusResponse{Status: osUpdateStatusProto(value)}, nil
}

func (s *Service) CheckOSUpdate(ctx context.Context, _ *sodav2.CheckOSUpdateRequest) (*sodav2.CheckOSUpdateResponse, error) {
	if s.osUpdates == nil {
		return nil, status.Error(codes.Unavailable, "OS update service is unavailable")
	}
	value, err := s.osUpdates.Check(ctx)
	if err != nil {
		return nil, osUpdateRPCError(err)
	}
	return &sodav2.CheckOSUpdateResponse{Release: &sodav2.OSRelease{ImageReference: value.ImageReference, Version: value.Version, SourceRevision: value.SourceRevision, FedoraBaseReference: value.FedoraBaseReference, Digest: value.Digest, StateSchema: value.StateSchema, Available: value.Available}}, nil
}

func (s *Service) StageOSUpdate(ctx context.Context, request *sodav2.StageOSUpdateRequest) (*sodav2.StageOSUpdateResponse, error) {
	if s.osUpdates == nil {
		return nil, status.Error(codes.Unavailable, "OS update service is unavailable")
	}
	value, err := s.osUpdates.Stage(ctx, request.GetImageReference())
	if err != nil {
		return nil, osUpdateRPCError(err)
	}
	return &sodav2.StageOSUpdateResponse{Status: osUpdateStatusProto(value)}, nil
}

func (s *Service) ActivateOSUpdate(ctx context.Context, request *sodav2.ActivateOSUpdateRequest) (*sodav2.ActivateOSUpdateResponse, error) {
	if s.osUpdates == nil {
		return nil, status.Error(codes.Unavailable, "OS update service is unavailable")
	}
	if !request.GetConfirmReboot() {
		return nil, status.Error(codes.InvalidArgument, "explicit maintenance reboot confirmation is required")
	}
	if err := s.osUpdates.Activate(ctx); err != nil {
		return nil, osUpdateRPCError(err)
	}
	return &sodav2.ActivateOSUpdateResponse{RebootRequested: true}, nil
}

func osUpdateStatusProto(value osupdate.Status) *sodav2.OSUpdateStatus {
	result := &sodav2.OSUpdateStatus{ReadOnly: value.ReadOnly}
	if value.Booted != nil {
		result.Booted = osDeploymentProto(value.Booted)
	}
	if value.Staged != nil {
		result.Staged = osDeploymentProto(value.Staged)
	}
	return result
}

func osDeploymentProto(value *osupdate.Deployment) *sodav2.OSDeployment {
	return &sodav2.OSDeployment{ImageReference: value.ImageReference, Version: value.Version, Digest: value.Digest, Architecture: value.Architecture, Incompatible: value.Incompatible, DownloadOnly: value.DownloadOnly}
}

func osUpdateRPCError(err error) error {
	switch {
	case errors.Is(err, osupdate.ErrInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, osupdate.ErrPrecondition), errors.Is(err, osupdate.ErrRejected):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "OS update request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "OS update request timed out")
	default:
		return status.Error(codes.Unavailable, "OS update service is unavailable")
	}
}

func validateUsername(value string) error {
	if err := domain.ValidateUnixIdentifier(value); err != nil {
		return invalid("username %s", err)
	}
	return nil
}

func validatePerson(username, displayName, email string) error {
	if err := validateUsername(username); err != nil {
		return err
	}
	if strings.TrimSpace(displayName) == "" {
		return invalid("display name is required")
	}
	if !strings.Contains(email, "@") {
		return invalid("email address is invalid")
	}
	return nil
}

func parseID(value, kind string) (string, error) {
	if _, err := uuid.Parse(value); err != nil {
		return "", invalid("invalid %s ID", kind)
	}
	return value, nil
}
