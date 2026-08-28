package telemetry

import (
	"context"
	"errors"
	"testing"
)

type fakeHostFiles map[string]string

func (f fakeHostFiles) ReadFile(path string) ([]byte, error) {
	value, ok := f[path]
	if !ok {
		return nil, errors.New("missing")
	}
	return []byte(value), nil
}
func (fakeHostFiles) Statfs(string) (uint64, uint64, error) { return 100, 40, nil }

type fakeRunner struct {
	ip  []byte
	err error
}

func (f fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	if name == "ip" {
		return f.ip, f.err
	}
	return nil, f.err
}

func TestHostParsersAndCPUChange(t *testing.T) {
	files := hostFixtureFiles()
	total, available := memoryStatus(files)
	if got := [2]uint64{total, available}; got != [2]uint64{10 * 1024, 3 * 1024} {
		t.Fatalf("bad memory parser: %v", got)
	}
	if load := loadAverage(files); load != [3]float64{1, 2, 3} || uptimeSeconds(files) != 12 {
		t.Fatalf("bad load/uptime parser: %v %d", load, uptimeSeconds(files))
	}
	sampler := NewSystemHostSampler(fakeRunner{}, files)
	if cpu := sampler.cpuPercent(); cpu != nil {
		t.Fatalf("first CPU sample should be nil, got %v", *cpu)
	}
	files["/proc/stat"] = "cpu  110 0 0 130 0\n"
	if cpu := sampler.cpuPercent(); cpu == nil || *cpu != 25 {
		t.Fatalf("unexpected CPU percentage: %v", cpu)
	}
	sampler = NewSystemHostSampler(fakeRunner{ip: []byte("[{\"ifname\":\"lo\",\"addr_info\":[]},{\"ifname\":\"eth0\",\"addr_info\":[{\"scope\":\"global\",\"local\":\"192.0.2.2\"}]}]")}, files)
	interfaces, err := networkInterfaces(context.Background(), sampler.Commands)
	if err != nil || len(interfaces) != 1 || interfaces[0].Name != "eth0" {
		t.Fatalf("bad interface parser: %#v %v", interfaces, err)
	}
}

func hostFixtureFiles() fakeHostFiles {
	return fakeHostFiles{
		"/proc/meminfo": "MemTotal:       10 kB\nMemAvailable: 3 kB\n",
		"/proc/loadavg": "1.0 2.0 3.0 1/2 3\n",
		"/proc/uptime":  "12.4 5.0\n",
		"/proc/stat":    "cpu  100 0 0 100 0\n",
	}
}

func TestNetworkParseFailureIsReported(t *testing.T) {
	_, err := networkInterfaces(context.Background(), fakeRunner{ip: []byte("not JSON")})
	if err == nil {
		t.Fatal("invalid JSON must be an observable parser failure")
	}
}
