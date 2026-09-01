package daemon

import (
	"context"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/version"
)

type Telemetry interface {
	HostStatus(context.Context) (*sodav2.HostStatus, error)
}

type Service struct {
	sodav2.UnimplementedSodaServiceServer
	telemetry Telemetry
}

type Options struct {
	Telemetry Telemetry
}

func New(options Options) *Service {
	return &Service{telemetry: options.Telemetry}
}

func (s *Service) Health(_ context.Context, _ *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ok", Service: "sodad", Version: version.Version}, nil
}
