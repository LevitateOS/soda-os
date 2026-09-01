package daemon

import (
	"context"
	"errors"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/version"
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

type Service struct {
	sodav2.UnimplementedSodaServiceServer
	telemetry Telemetry
	osUpdates OSUpdater
}

type Options struct {
	Telemetry Telemetry
	OSUpdates OSUpdater
}

func New(options Options) *Service {
	return &Service{telemetry: options.Telemetry, osUpdates: options.OSUpdates}
}

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
