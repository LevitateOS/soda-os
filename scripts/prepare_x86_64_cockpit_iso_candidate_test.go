package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestPrepareX86CockpitISORefusesNonX86Host(t *testing.T) {
	root, bin, log := prepareScriptTest(t, "aarch64")
	cmd := scriptCommand(t, root, bin, log)
	err := cmd.Run()
	if err == nil {
		t.Fatal("script succeeded on non-x86_64 host")
	}
	if !strings.Contains(readFile(t, log), "uname -m") {
		t.Fatal("script did not check host architecture first")
	}
}

func TestPrepareX86CockpitISORefusesExistingDestination(t *testing.T) {
	root, bin, log := prepareScriptTest(t, "x86_64")
	destination := filepath.Join(root, "libvirt", "SodaOS-0.6.3-x86_64.iso")
	if err := os.WriteFile(destination, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := scriptCommand(t, root, bin, log)
	err := cmd.Run()
	if err == nil {
		t.Fatal("script overwrote an existing destination")
	}
	commands := readFile(t, log)
	if strings.Contains(commands, "just check") {
		t.Fatalf("script reached artifact mutation after destination collision:\n%s", commands)
	}
}

func TestPrepareX86CockpitISOCommandSequence(t *testing.T) {
	root, bin, log := prepareScriptTest(t, "x86_64")
	cmd := scriptCommand(t, root, bin, log)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s\nlog:\n%s", err, output, readFile(t, log))
	}
	commands := readFile(t, log)
	wantOrder := []string{
		"uname -m",
		"git status --porcelain=v1 --untracked-files=all",
		"git rev-parse HEAD",
		"git rev-parse origin/main",
		"skopeo inspect --no-creds docker://ghcr.io/levitateos/soda-os:sha-" + testRevision + "-x86_64",
		"just check",
		"just rpm x86_64",
		"just oci x86_64",
		"go run ./cmd/soda-release image-stage --architecture x86_64 --archive .artifacts/images/soda-os-0.6.3-x86_64.oci.tar",
		"skopeo inspect --format {{.Digest}} oci-archive:.artifacts/images/soda-os-0.6.3-x86_64.oci.tar",
		"skopeo inspect --format {{.Digest}} --no-creds docker://ghcr.io/levitateos/soda-os:sha-" + testRevision + "-x86_64",
		"just iso x86_64 .artifacts/images/soda-os-0.6.3-x86_64.oci.tar",
		"docker load --input .artifacts/images/soda-os-0.6.3-x86_64.oci.tar",
	}
	last := -1
	for _, want := range wantOrder {
		index := strings.Index(commands, want)
		if index < 0 {
			t.Fatalf("missing command %q in log:\n%s", want, commands)
		}
		if index < last {
			t.Fatalf("command %q ran out of order:\n%s", want, commands)
		}
		last = index
	}
	for _, forbidden := range []string{" aarch64", "qcow2", "record-sign", "image-promote"} {
		if strings.Contains(commands, forbidden) {
			t.Fatalf("script ran forbidden operation %q:\n%s", forbidden, commands)
		}
	}
	if !strings.Contains(string(output), "Cockpit-selectable ISO: "+filepath.Join(root, "libvirt", "SodaOS-0.6.3-x86_64.iso")) {
		t.Fatalf("summary did not include Cockpit path:\n%s", output)
	}
}

func TestPrepareX86CockpitISOProgress(t *testing.T) {
	root, bin, log := prepareScriptTest(t, "x86_64")
	cmd := scriptCommand(t, root, bin, log)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, stderr.String())
	}
	want := []string{
		"Checking native x86_64 host and required commands",
		"Checking clean origin/main source identity and deriving Soda version",
		"Checking candidate tag and destination availability",
		"Checking libvirt destination traversal permissions",
		"Running just check",
		"Building locked x86_64 RPM inputs",
		"Building x86_64 OCI archive",
		"Publishing immutable GHCR candidate with soda-release image-stage",
		"Verifying anonymous GHCR digest against local OCI digest",
		"Verifying OCI platform, Soda version, and exact source revision",
		"Building and inspecting matching graphical network-install ISO",
		"Verifying ISO checksum sidecar and exact installer source",
		"Verifying installed OCI os-release and Soda RPM versions",
		"Copying ISO without overwrite to " + filepath.Join(root, "libvirt", "SodaOS-0.6.3-x86_64.iso"),
		"Comparing source and copied ISO SHA-256 checksums",
		"Verifying libvirt traversal, virt_image_t label, and non-booting QEMU open as qemu",
	}
	expected := "==> " + strings.Join(want, "\n==> ") + "\n"
	if stderr.String() != expected {
		t.Fatalf("unexpected progress output:\n%s", stderr.String())
	}
	if strings.Contains(string(stdout), "==>") {
		t.Fatalf("progress leaked onto stdout:\n%s", stdout)
	}
}

func scriptCommand(t *testing.T, root, bin, log string) *exec.Cmd {
	t.Helper()
	script, err := filepath.Abs("prepare-x86_64-cockpit-iso-candidate.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+":"+os.Getenv("PATH"),
		"SODA_COCKPIT_ISO_DESTINATION_DIR="+filepath.Join(root, "libvirt"),
		"SODA_SCRIPT_TEST_ROOT="+root,
		"SODA_SCRIPT_TEST_LOG="+log,
	)
	return cmd
}

func prepareScriptTest(t *testing.T, machine string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	log := filepath.Join(root, "commands.log")
	for _, dir := range []string{bin, filepath.Join(root, "distro"), filepath.Join(root, "libvirt")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "distro", "soda.toml"), []byte("[identity]\nversion = \"0.6.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStub(t, bin, "uname", "#!/usr/bin/env bash\necho \"uname $*\" >>\"$SODA_SCRIPT_TEST_LOG\"\necho "+machine+"\n")
	writeStub(t, bin, "git", `#!/usr/bin/env bash
printf 'git' >>"$SODA_SCRIPT_TEST_LOG"; printf ' %s' "$@" >>"$SODA_SCRIPT_TEST_LOG"; printf '\n' >>"$SODA_SCRIPT_TEST_LOG"
case "$*" in
  'rev-parse --show-toplevel') echo "$SODA_SCRIPT_TEST_ROOT" ;;
  'status --porcelain=v1 --untracked-files=all') ;;
  'rev-parse HEAD'|'rev-parse origin/main') echo '`+testRevision+`' ;;
  *) echo "unexpected git $*" >&2; exit 2 ;;
esac
`)
	writeStub(t, bin, "skopeo", `#!/usr/bin/env bash
printf 'skopeo' >>"$SODA_SCRIPT_TEST_LOG"; printf ' %s' "$@" >>"$SODA_SCRIPT_TEST_LOG"; printf '\n' >>"$SODA_SCRIPT_TEST_LOG"
joined="$*"
if [[ "$joined" == 'inspect --no-creds docker://ghcr.io/levitateos/soda-os:sha-`+testRevision+`-x86_64' ]]; then
  [[ -f "$SODA_SCRIPT_TEST_ROOT/staged" ]] || exit 1
  echo '{"Digest":"sha256:`+strings.Repeat("a", 64)+`"}'
  exit 0
fi
case "$joined" in
  *'{{.Digest}}'*) echo 'sha256:`+strings.Repeat("a", 64)+`' ;;
  *'{{.Os}}'*) echo linux ;;
  *'{{.Architecture}}'*) echo amd64 ;;
  *'org.opencontainers.image.version'*) echo '0.6.3' ;;
  *'org.opencontainers.image.revision'*) echo '`+testRevision+`' ;;
  *) echo "unexpected skopeo $joined" >&2; exit 2 ;;
esac
`)
	writeStub(t, bin, "just", `#!/usr/bin/env bash
printf 'just' >>"$SODA_SCRIPT_TEST_LOG"; printf ' %s' "$@" >>"$SODA_SCRIPT_TEST_LOG"; printf '\n' >>"$SODA_SCRIPT_TEST_LOG"
case "$*" in
  check|'rpm x86_64') ;;
  'oci x86_64') mkdir -p .artifacts/images; printf oci > .artifacts/images/soda-os-0.6.3-x86_64.oci.tar ;;
  'iso x86_64 .artifacts/images/soda-os-0.6.3-x86_64.oci.tar')
    mkdir -p .artifacts/images .artifacts/installer/context
    printf iso > .artifacts/images/SodaOS-0.6.3-x86_64.iso
    sum=$(/usr/bin/sha256sum .artifacts/images/SodaOS-0.6.3-x86_64.iso | awk '{print $1}')
    printf '%s  SodaOS-0.6.3-x86_64.iso\n' "$sum" > .artifacts/images/SodaOS-0.6.3-x86_64.iso.sha256
    printf 'bootc --source-imgref="docker://ghcr.io/levitateos/soda-os@sha256:`+strings.Repeat("a", 64)+`" --target-imgref="ghcr.io/levitateos/soda-os@sha256:`+strings.Repeat("a", 64)+`"\n' > .artifacts/installer/context/interactive-defaults.ks ;;
  *) echo "unexpected just $*" >&2; exit 2 ;;
esac
`)
	writeStub(t, bin, "go", `#!/usr/bin/env bash
printf 'go' >>"$SODA_SCRIPT_TEST_LOG"; printf ' %s' "$@" >>"$SODA_SCRIPT_TEST_LOG"; printf '\n' >>"$SODA_SCRIPT_TEST_LOG"
[[ "$*" == 'run ./cmd/soda-release image-stage --architecture x86_64 --archive .artifacts/images/soda-os-0.6.3-x86_64.oci.tar' ]] || exit 2
touch "$SODA_SCRIPT_TEST_ROOT/staged"
`)
	writeStub(t, bin, "docker", `#!/usr/bin/env bash
printf 'docker' >>"$SODA_SCRIPT_TEST_LOG"; printf ' %s' "$@" >>"$SODA_SCRIPT_TEST_LOG"; printf '\n' >>"$SODA_SCRIPT_TEST_LOG"
`)
	writeStub(t, bin, "sudo", `#!/usr/bin/env bash
printf 'sudo' >>"$SODA_SCRIPT_TEST_LOG"; printf ' %s' "$@" >>"$SODA_SCRIPT_TEST_LOG"; printf '\n' >>"$SODA_SCRIPT_TEST_LOG"
`)
	writeStub(t, bin, "stat", "#!/usr/bin/env bash\necho \"stat $*\" >>\"$SODA_SCRIPT_TEST_LOG\"\necho 'system_u:object_r:virt_image_t:s0'\n")
	writeStub(t, bin, "qemu-system-x86_64", "#!/usr/bin/env bash\necho qemu-system-x86_64 >>\"$SODA_SCRIPT_TEST_LOG\"\n")
	return root, bin, log
}

func writeStub(t *testing.T, bin, name, content string) {
	t.Helper()
	path := filepath.Join(bin, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(contents)
}
