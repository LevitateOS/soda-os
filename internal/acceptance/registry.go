package acceptance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Docker struct {
	Prefix []string
}

type Registry struct {
	Docker   Docker
	Name     string
	Port     int
	Data     string
	Evidence Evidence
}

func SelectDocker(ctx context.Context) (Docker, error) {
	if err := RunCommand(ctx, CommandSpec{Name: "docker", Args: []string{"info"}, Stdout: io.Discard, Stderr: io.Discard}); err == nil {
		return Docker{}, nil
	}
	if err := RunCommand(ctx, CommandSpec{Name: "sudo", Args: []string{"-n", "docker", "info"}, Stdout: io.Discard, Stderr: io.Discard}); err == nil {
		return Docker{Prefix: []string{"sudo", "-n"}}, nil
	}
	return Docker{}, errors.New("Docker is unavailable directly or through passwordless sudo")
}

func (docker Docker) Run(ctx context.Context, args ...string) error {
	name, commandArgs := docker.command(args)
	return RunCommand(ctx, CommandSpec{Name: name, Args: commandArgs})
}

func (docker Docker) Output(ctx context.Context, args ...string) ([]byte, error) {
	name, commandArgs := docker.command(args)
	return CommandOutput(ctx, CommandSpec{Name: name, Args: commandArgs})
}

func (docker Docker) command(args []string) (string, []string) {
	if len(docker.Prefix) == 0 {
		return "docker", args
	}
	return docker.Prefix[0], append(append([]string(nil), docker.Prefix[1:]...), append([]string{"docker"}, args...)...)
}

func (registry Registry) Start(ctx context.Context, image string) error {
	if err := os.MkdirAll(registry.Data, 0o700); err != nil {
		return err
	}
	output, err := registry.Docker.Output(ctx, "run", "--detach", "--name", registry.Name,
		"--publish", fmt.Sprintf("127.0.0.1:%d:5000", registry.Port),
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--volume", registry.Data+":/var/lib/registry", image)
	if err != nil {
		return fmt.Errorf("start disposable registry: %w", err)
	}
	if err = registry.Evidence.Write("registry-container-id.txt", output); err != nil {
		return errors.Join(err, registry.stopAfterFailedStart())
	}
	if err = registry.waitReady(ctx); err != nil {
		return errors.Join(err, registry.stopAfterFailedStart())
	}
	return nil
}

func (registry Registry) stopAfterFailedStart() error {
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return registry.Stop(stopCtx)
}

func (registry Registry) waitReady(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/", registry.Port)
	for {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := RunCommand(requestCtx, CommandSpec{Name: "curl", Args: []string{"--fail", "--silent", "--show-error", url}, Stdout: io.Discard, Stderr: io.Discard})
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for disposable registry: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (registry Registry) Stop(ctx context.Context) error {
	if err := registry.Docker.Run(ctx, "rm", "-f", registry.Name); err != nil {
		return fmt.Errorf("remove disposable registry %s: %w", registry.Name, err)
	}
	return nil
}

func (registry Registry) Publish(ctx context.Context, archive, tag, skopeoImage string) (string, error) {
	if err := requireRegularFile(archive); err != nil {
		return "", err
	}
	if _, err := exec.LookPath("skopeo"); err == nil {
		return registry.publishNative(ctx, archive, tag)
	}
	return registry.publishContainer(ctx, archive, tag, skopeoImage)
}

func (registry Registry) publishNative(ctx context.Context, archive, tag string) (string, error) {
	repository := fmt.Sprintf("127.0.0.1:%d/soda-os", registry.Port)
	copyOutput, err := CommandOutput(ctx, CommandSpec{Name: "skopeo", Args: []string{
		"copy", "--preserve-digests", "--src-no-creds", "--dest-no-creds", "--dest-tls-verify=false",
		"oci-archive:" + archive, "docker://" + repository + ":" + tag,
	}})
	if err != nil {
		return "", err
	}
	_ = registry.Evidence.Write(filepath.Join("registry", tag+"-copy.txt"), copyOutput)
	return registry.inspectNative(ctx, repository, tag)
}

func (registry Registry) inspectNative(ctx context.Context, repository, tag string) (string, error) {
	output, err := CommandOutput(ctx, CommandSpec{Name: "skopeo", Args: []string{
		"inspect", "--no-creds", "--tls-verify=false", "--format", "{{.Digest}}", "docker://" + repository + ":" + tag,
	}})
	return strings.TrimSpace(string(output)), err
}

func (registry Registry) publishContainer(ctx context.Context, archive, tag, image string) (string, error) {
	if image == "" {
		return "", errors.New("containerized Skopeo image is required when Skopeo is unavailable")
	}
	network, host, err := skopeoContainerNetwork()
	if err != nil {
		return "", err
	}
	repository := fmt.Sprintf("%s:%d/soda-os", host, registry.Port)
	base := []string{"run", "--rm"}
	base = append(base, network...)
	base = append(base, "--volume", archive+":/input/archive.tar:ro", "--entrypoint", "/usr/bin/skopeo", image)
	copyArgs := append(append([]string(nil), base...), "copy", "--preserve-digests", "--src-no-creds", "--dest-no-creds", "--dest-tls-verify=false", "oci-archive:/input/archive.tar", "docker://"+repository+":"+tag)
	if _, err = registry.Docker.Output(ctx, copyArgs...); err != nil {
		return "", err
	}
	inspectArgs := append(append([]string(nil), base...), "inspect", "--no-creds", "--tls-verify=false", "--format", "{{.Digest}}", "docker://"+repository+":"+tag)
	output, err := registry.Docker.Output(ctx, inspectArgs...)
	return strings.TrimSpace(string(output)), err
}

func skopeoContainerNetwork() ([]string, string, error) {
	switch runtime.GOOS {
	case "linux":
		return []string{"--network", "host"}, "127.0.0.1", nil
	case "darwin":
		return nil, "host.docker.internal", nil
	default:
		return nil, "", fmt.Errorf("containerized Skopeo is unsupported on %s", runtime.GOOS)
	}
}

func ReadPinnedImage(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(contents))
	if line == "" || strings.ContainsAny(line, "\r\n") {
		return "", fmt.Errorf("pinned image file %s must contain one line", path)
	}
	return line, nil
}

func RegistryName(now time.Time) string {
	return "soda-acceptance-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(now.UnixNano(), 10)
}
