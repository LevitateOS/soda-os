package main

import sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"

func healthJSON(health *sodav2.HealthResponse) any {
	if health == nil {
		return nil
	}
	return map[string]any{"status": health.Status, "service": health.Service, "version": health.Version}
}

func osDeploymentJSON(deployment *sodav2.OSDeployment) any {
	if deployment == nil {
		return nil
	}
	return map[string]any{"image_reference": deployment.ImageReference, "version": deployment.Version, "digest": deployment.Digest, "architecture": deployment.Architecture, "incompatible": deployment.Incompatible, "download_only": deployment.DownloadOnly}
}

func osUpdateStatusJSON(value *sodav2.OSUpdateStatus) any {
	if value == nil {
		return nil
	}
	return map[string]any{"booted": osDeploymentJSON(value.Booted), "staged": osDeploymentJSON(value.Staged), "read_only": value.ReadOnly}
}

func osReleaseJSON(value *sodav2.OSRelease) any {
	if value == nil {
		return nil
	}
	return map[string]any{"image_reference": value.ImageReference, "version": value.Version, "source_revision": value.SourceRevision, "fedora_base_reference": value.FedoraBaseReference, "digest": value.Digest, "state_schema": value.StateSchema, "available": value.Available}
}
