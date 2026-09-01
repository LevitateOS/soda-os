package daemon

import (
	"context"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/version"
)

type Service struct {
	sodav2.UnimplementedSodaServiceServer
}

func New() *Service {
	return &Service{}
}

func (s *Service) Health(_ context.Context, _ *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ok", Service: "sodad", Version: version.Version}, nil
}
