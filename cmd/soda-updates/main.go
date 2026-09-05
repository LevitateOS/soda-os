// soda-updates is a synchronous, administrator-only command for the Cockpit
// Updates page. It owns no daemon, deployment database, or background jobs.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/LevitateOS/soda-os/internal/updates"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func main() {
	architecture := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	command := newCommand(process.OSRunner{Stderr: os.Stderr}, architecture, os.Geteuid())
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "soda-updates:", err)
		os.Exit(1)
	}
}

func newCommand(runner process.Runner, architecture string, euid int) *cobra.Command {
	root := &cobra.Command{Use: "soda-updates", Short: "Check and apply approved Soda images through native bootc", SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			if euid != 0 {
				return errors.New("enable Cockpit administrative access to manage Soda Updates")
			}
			if architecture == "" {
				return errors.New("Soda Updates requires x86_64 or aarch64")
			}
			return nil
		}}
	root.CompletionOptions.DisableDefaultCmd = true
	releases := updates.NewReleases(runner)
	root.AddCommand(&cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		host, err := updates.ReadHost(cmd.Context(), runner)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(host)
	}})
	root.AddCommand(&cobra.Command{Use: "check", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
		defer cancel()
		selected, err := releases.Latest(ctx, architecture)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(selected)
	}})
	operations := updates.Operations{Runner: process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr}, Releases: releases, Architecture: architecture}
	root.AddCommand(operationCommand("download", operations.Download), operationCommand("apply", operations.Apply))
	return root
}

func operationCommand(name string, run func(context.Context, updates.Selection) error) *cobra.Command {
	var selection updates.Selection
	command := &cobra.Command{Use: name, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		lock, err := os.OpenFile("/run/soda-updates.lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0o600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return errors.New("another Soda update operation is running; refresh when it finishes")
		}
		defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		return run(cmd.Context(), selection)
	}}
	command.Flags().StringVar(&selection.Version, "version", "", "confirmed published Soda version")
	command.Flags().StringVar(&selection.Reference, "reference", "", "confirmed exact Soda image digest")
	_ = command.MarkFlagRequired("version")
	_ = command.MarkFlagRequired("reference")
	return command
}
