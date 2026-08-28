package sodactl

import (
	"context"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (a *App) osCommand(socket *string) *cobra.Command {
	osCommand := &cobra.Command{Use: "os", Short: "Administer the Soda OS base image"}
	update := &cobra.Command{Use: "update", Short: "Manually check, stage, and activate OS updates"}
	update.AddCommand(a.osStatusCommand(socket), a.osCheckCommand(socket), a.osStageCommand(socket), a.osActivateCommand(socket))
	osCommand.AddCommand(update)
	return osCommand
}

func (a *App) osStatusCommand(socket *string) *cobra.Command {
	return &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.callWithTimeout(cmd, *socket, defaultOSReadTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.GetOSUpdateStatus(ctx, &sodav2.GetOSUpdateStatusRequest{})
			return osUpdateStatusJSON(response.GetStatus()), err
		})
	}}
}

func (a *App) osCheckCommand(socket *string) *cobra.Command {
	return &cobra.Command{Use: "check", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.callWithTimeout(cmd, *socket, defaultOSReadTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
			return osReleaseJSON(response.GetRelease()), err
		})
	}}
}

func (a *App) osStageCommand(socket *string) *cobra.Command {
	return &cobra.Command{Use: "stage", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.callWithTimeout(cmd, *socket, defaultOSStageTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			checked, err := client.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
			if err != nil {
				return nil, err
			}
			release := checked.GetRelease()
			if release == nil || !release.GetAvailable() {
				return nil, status.Error(codes.FailedPrecondition, "no newer signed Soda OS release is available")
			}
			response, err := client.StageOSUpdate(ctx, &sodav2.StageOSUpdateRequest{ImageReference: release.GetImageReference()})
			return osUpdateStatusJSON(response.GetStatus()), err
		})
	}}
}

func (a *App) osActivateCommand(socket *string) *cobra.Command {
	var confirmReboot bool
	command := &cobra.Command{Use: "activate", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.callWithTimeout(cmd, *socket, defaultOSActivateTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{ConfirmReboot: confirmReboot})
			return map[string]any{"reboot_requested": response.GetRebootRequested()}, err
		})
	}}
	command.Flags().BoolVar(&confirmReboot, "confirm-reboot", false, "confirm immediate maintenance reboot into the staged image")
	_ = command.MarkFlagRequired("confirm-reboot")
	return command
}
