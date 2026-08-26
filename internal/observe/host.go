package observe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

// CommandRunner is intentionally small to make command failure behavior
// testable without a running Soda host.
type CommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type OSCommandRunner struct{}

func (OSCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return runCommand(ctx, name, args...)
}

type HostFiles interface {
	ReadFile(string) ([]byte, error)
	Statfs(string) (total uint64, available uint64, err error)
}

type OSHostFiles struct{}

func (OSHostFiles) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (OSHostFiles) Statfs(path string) (uint64, uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	return stat.Blocks * uint64(stat.Bsize), stat.Bavail * uint64(stat.Bsize), nil
}

// SystemHostSampler implements Soda's read-only host snapshot from ordinary
// Linux interfaces. Missing optional host facilities degrade the snapshot
// rather than making the daemon unavailable.
type SystemHostSampler struct {
	Commands CommandRunner
	Files    HostFiles
	mu       sync.Mutex
	cpu      *cpuSample
}

type cpuSample struct{ total, idle uint64 }

func NewSystemHostSampler(commands CommandRunner, files HostFiles) *SystemHostSampler {
	if commands == nil {
		commands = OSCommandRunner{}
	}
	if files == nil {
		files = OSHostFiles{}
	}
	return &SystemHostSampler{Commands: commands, Files: files}
}

func (s *SystemHostSampler) SampleHost(ctx context.Context) (domain.HostStatus, error) {
	services := make([]domain.ServiceStatus, 0, 7)
	for _, name := range []string{"sodad", "soda-authd", "soda-cockpit", "sshd", "avahi-daemon", "NetworkManager", "firewalld"} {
		state := domain.RuntimeReady // this process is the reporting sodad.
		if name != "sodad" {
			state = serviceState(ctx, s.Commands, name)
		}
		services = append(services, domain.ServiceStatus{Name: name, State: state})
	}
	interfaces, interfaceErr := networkInterfaces(ctx, s.Commands)
	memTotal, memAvailable := memoryStatus(s.Files)
	load := loadAverage(s.Files)
	uptime := uptimeSeconds(s.Files)
	cpu := s.cpuPercent()
	filesystems := make([]domain.FilesystemStatus, 0, 3)
	for _, path := range []string{"/", "/srv/soda/projects", "/opt/soda/toolchains"} {
		if total, available, err := s.Files.Statfs(path); err == nil {
			filesystems = append(filesystems, domain.FilesystemStatus{Path: path, TotalBytes: total, AvailableBytes: available})
		}
	}
	ssh := firewallReady(ctx, s.Commands, "--query-service", "ssh")
	cockpit := firewallReady(ctx, s.Commands, "--query-port", "9090/tcp")
	overall := domain.RuntimeReady
	if interfaceErr != nil || !ssh || !cockpit || len(interfaces) == 0 {
		overall = domain.RuntimeDegraded
	}
	for _, service := range services {
		if service.State != domain.RuntimeReady {
			overall = domain.RuntimeDegraded
			break
		}
	}
	return domain.HostStatus{
		SampledAt:            time.Now(),
		Overall:              overall,
		Services:             services,
		SSHFirewallReady:     ssh,
		CockpitFirewallReady: cockpit,
		Interfaces:           interfaces,
		CPUPercent:           cpu,
		LoadAverage:          load,
		UptimeSeconds:        uptime,
		MemoryTotalBytes:     memTotal,
		MemoryAvailableBytes: memAvailable,
		Filesystems:          filesystems,
	}, nil
}

func serviceState(ctx context.Context, runner CommandRunner, name string) domain.RuntimeState {
	_, err := runner.Output(ctx, "systemctl", "is-active", "--quiet", name)
	if err == nil {
		return domain.RuntimeReady
	}
	if errors.Is(err, os.ErrNotExist) {
		return domain.RuntimeUnavailable
	}
	return domain.RuntimeDegraded
}

func firewallReady(ctx context.Context, runner CommandRunner, flag, value string) bool {
	_, err := runner.Output(ctx, "firewall-cmd", "--quiet", flag, value)
	return err == nil
}

type ipAddress struct {
	Name      string `json:"ifname"`
	Addresses []struct {
		Scope string `json:"scope"`
		Local string `json:"local"`
	} `json:"addr_info"`
}

func networkInterfaces(ctx context.Context, runner CommandRunner) ([]domain.NetworkInterface, error) {
	output, err := runner.Output(ctx, "ip", "-json", "address", "show", "up")
	if err != nil {
		return nil, err
	}
	var addresses []ipAddress
	if err := json.Unmarshal(output, &addresses); err != nil {
		return nil, fmt.Errorf("parse ip JSON: %w", err)
	}
	interfaces := make([]domain.NetworkInterface, 0, len(addresses))
	for _, item := range addresses {
		if item.Name == "" || item.Name == "lo" {
			continue
		}
		var globals []string
		for _, address := range item.Addresses {
			if address.Scope == "global" && address.Local != "" {
				globals = append(globals, address.Local)
			}
		}
		if len(globals) > 0 {
			interfaces = append(interfaces, domain.NetworkInterface{Name: item.Name, Addresses: globals})
		}
	}
	return interfaces, nil
}

func memoryStatus(files HostFiles) (uint64, uint64) {
	contents, _ := files.ReadFile("/proc/meminfo")
	find := func(key string) uint64 {
		for _, line := range strings.Split(string(contents), "\n") {
			if rest, ok := strings.CutPrefix(line, key); ok {
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					value, _ := strconv.ParseUint(fields[0], 10, 64)
					return value * 1024
				}
			}
		}
		return 0
	}
	return find("MemTotal:"), find("MemAvailable:")
}

func loadAverage(files HostFiles) [3]float64 {
	contents, _ := files.ReadFile("/proc/loadavg")
	fields := strings.Fields(string(contents))
	var result [3]float64
	for i := range result {
		if i < len(fields) {
			result[i], _ = strconv.ParseFloat(fields[i], 64)
		}
	}
	return result
}

func uptimeSeconds(files HostFiles) uint64 {
	contents, _ := files.ReadFile("/proc/uptime")
	field, _, _ := strings.Cut(string(contents), " ")
	value, _ := strconv.ParseFloat(field, 64)
	return uint64(value)
}

func (s *SystemHostSampler) cpuPercent() *float64 {
	contents, err := s.Files.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}
	line, _, _ := strings.Cut(string(contents), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return nil
	}
	var total uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil
		}
		total += value
	}
	idle, _ := strconv.ParseUint(fields[4], 10, 64)
	if len(fields) > 5 {
		iowait, _ := strconv.ParseUint(fields[5], 10, 64)
		idle += iowait
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.cpu
	s.cpu = &cpuSample{total: total, idle: idle}
	if previous == nil || total <= previous.total {
		return nil
	}
	deltaTotal := total - previous.total
	deltaIdle := idle - previous.idle
	percent := 100 * float64(deltaTotal-deltaIdle) / float64(deltaTotal)
	return &percent
}

func pathExists(path string) bool {
	_, err := os.Stat(filepath.Clean(path))
	return err == nil
}
