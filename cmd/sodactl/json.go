package main

import sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"

func healthJSON(health *sodav2.HealthResponse) any {
	if health == nil {
		return nil
	}
	return map[string]any{"status": health.Status, "service": health.Service, "version": health.Version}
}
