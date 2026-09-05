package scripts

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const testImageID = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

type nativeScriptFixture struct {
	root string
	bin  string
	log  string
	arch string
}

func TestPrepareNativeISOSequence(t *testing.T) {
	for _, arch := range []string{"aarch64", "x86_64"} {
		t.Run(arch, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, arch, "Linux")
			output := runScriptOK(t, fixture.command(t, "prepare-native-iso-candidate.sh", arch))
			commands := readFile(t, fixture.log)
			requireOrder(t, commands, "docker info", "docker buildx ls", "skopeo list-tags", "just check", "just oci "+arch,
				"skopeo inspect --raw", "docker load", "docker image inspect", "docker run", "go run ./cmd/soda-release image-stage",
				"skopeo inspect --no-creds", "just iso "+arch)
			for _, forbidden := range []string{"just rpm", "sudo ", "qcow2", "image-promote", "record-sign"} {
				require.NotContains(t, commands, forbidden)
			}
			for _, expected := range []string{"--pull=never --platform linux/", "--entrypoint /bin/sh " + testImageID, "context=native builder=native"} {
				require.Contains(t, commands, expected)
			}
			require.Contains(t, output, "Publication succeeded:")
			require.Contains(t, output, "not installation-tested")
			require.Contains(t, output, testDigest)
		})
	}
}

func TestPrepareNativeISOUsesExactContainerdManifestID(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
	cmd := fixture.command(t, "prepare-native-iso-candidate.sh", fixture.arch)
	cmd.Env = append(cmd.Env, "SODA_TEST_IMAGE_STORE=containerd")
	runScriptOK(t, cmd)
	commands := readFile(t, fixture.log)
	require.Contains(t, commands, "image inspect --format {{.Id}} ghcr.io/levitateos/soda-os:0.6.3")
	require.Contains(t, commands, "--entrypoint /bin/sh "+testDigest)
	require.NotContains(t, commands, "--entrypoint /bin/sh ghcr.io/")
}

func TestPrepareNativeISOCanBeSourcedWithoutPublication(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
	writeTestFile(t, filepath.Join(fixture.root, "scripts", "source-only.sh"), "#!/usr/bin/env bash\nsource scripts/prepare-native-iso-candidate.sh\n", 0o755)
	runScriptOK(t, fixture.command(t, "source-only.sh"))
	_, err := os.Stat(fixture.log)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestPrepareNativeISORefusesCollisions(t *testing.T) {
	for _, suffix := range []string{"soda-os-0.6.3-aarch64.oci.tar", "SodaOS-0.6.3-aarch64.iso", "SodaOS-0.6.3-aarch64.iso.sha256"} {
		t.Run(suffix, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
			path := filepath.Join(fixture.root, ".artifacts", "images", suffix)
			if err := os.Symlink("missing-target", path); err != nil {
				t.Fatal(err)
			}
			runScriptFails(t, fixture.command(t, "prepare-native-iso-candidate.sh", fixture.arch), "output already exists")
			if strings.Contains(readFile(t, fixture.log), "just ") {
				t.Fatal("collision reached checks or construction")
			}
		})
	}
}

func TestPrepareNativeISOPrepublicationFailures(t *testing.T) {
	for _, failure := range []string{"dirty", "git-status", "status-after-check", "origin", "tag", "tag-network", "check", "oci", "metadata", "config-digest", "manifest-digest", "image-id", "runtime", "drift-check", "drift-oci"} {
		t.Run(failure, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
			cmd := fixture.command(t, "prepare-native-iso-candidate.sh", fixture.arch)
			cmd.Env = append(cmd.Env, "SODA_TEST_FAIL="+failure)
			runScriptFails(t, cmd, "")
			if strings.Contains(readFile(t, fixture.log), "go run ./cmd/soda-release") {
				t.Fatal("prepublication failure reached publication")
			}
		})
	}
}

func TestPrepareNativeISOReportsPartialPublication(t *testing.T) {
	for _, failure := range []string{"stage", "remote-digest", "remote-network", "iso", "checksum", "binding"} {
		t.Run(failure, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
			cmd := fixture.command(t, "prepare-native-iso-candidate.sh", fixture.arch)
			cmd.Env = append(cmd.Env, "SODA_TEST_FAIL="+failure)
			state := "confirmed"
			if failure == "stage" {
				state = "attempted"
			}
			output := runScriptFails(t, cmd, "publication="+state)
			require.Contains(t, output, testDigest)
			require.Contains(t, output, "sha-"+testRevision+"-aarch64")
			commands := readFile(t, fixture.log)
			require.Equal(t, 1, strings.Count(commands, "go run ./cmd/soda-release image-stage"), commands)
			require.NotContains(t, output, "candidate is ready")
		})
	}
}

func TestPrepareNativeISODarwinUsesLinuxChecks(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Darwin")
	runScriptOK(t, fixture.command(t, "prepare-native-iso-candidate.sh", fixture.arch))
	commands := readFile(t, fixture.log)
	requireOrder(t, commands, "docker build --platform linux/arm64", "git clone --no-local --no-checkout", "just oci aarch64")
	if strings.Contains(commands, "\njust check\n") {
		t.Fatal("Darwin ran host just check")
	}
}

func prepareNativeScriptTest(t *testing.T, arch, hostOS string) nativeScriptFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture := nativeScriptFixture{root: root, bin: filepath.Join(root, "bin"), log: filepath.Join(root, "commands.log"), arch: arch}
	for _, directory := range []string{fixture.bin, filepath.Join(root, "scripts"), filepath.Join(root, "distro"), filepath.Join(root, ".artifacts", "images"), filepath.Join(root, "destination")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, script := range []string{"check-native.sh", "prepare-native-iso-candidate.sh", "place-libvirt-iso.sh"} {
		writeTestFile(t, filepath.Join(root, "scripts", script), readFile(t, script), 0o755)
	}
	writeTestFile(t, filepath.Join(root, "distro", "soda.toml"), "[identity]\nversion = \"0.6.3\"\n", 0o644)
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/check\ngo 1.26.7\n", 0o644)
	writeStub(t, fixture.bin, "uname", "case \"$1\" in -m) echo "+arch+" ;; -s) echo "+hostOS+" ;; *) exit 2 ;; esac")
	writeStub(t, fixture.bin, "git", gitStub)
	writeStub(t, fixture.bin, "docker", dockerStub)
	writeStub(t, fixture.bin, "skopeo", skopeoStub)
	writeStub(t, fixture.bin, "just", justStub)
	writeStub(t, fixture.bin, "go", goStub)
	writeStub(t, fixture.bin, "sha256sum", `exec "$SODA_TEST_BINARY" -test.run=^TestNativeChecksumHelper$ -- "$@"`)
	for _, command := range []string{"vp", "qemu-system-aarch64", "qemu-system-x86_64"} {
		writeStub(t, fixture.bin, command, "exit 0")
	}
	writeStub(t, fixture.bin, "sudo", `[[ ${SODA_TEST_FAIL:-} != traversal || "$*" != *'test -x'* ]] || exit 1
[[ ${SODA_TEST_FAIL:-} != readable || "$*" != *'test -r'* ]] || exit 1
[[ ${SODA_TEST_FAIL:-} != qemu-open || "$*" != *'sh -eu -c'* ]] || exit 1`)
	writeStub(t, fixture.bin, "stat", `[[ ${SODA_TEST_FAIL:-} != label ]] || { echo unconfined_u:object_r:default_t:s0; exit 0; }
echo system_u:object_r:virt_image_t:s0`)
	return fixture
}

func (fixture nativeScriptFixture) command(t *testing.T, script string, arguments ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(filepath.Join(fixture.root, "scripts", script), arguments...)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Dir = fixture.root
	cmd.Env = append(os.Environ(), "PATH="+fixture.bin+":"+os.Getenv("PATH"), "SODA_TEST_ROOT="+fixture.root, "SODA_TEST_LOG="+fixture.log,
		"SODA_TEST_ARCH="+fixture.arch, "SODA_TEST_BINARY="+binary, "SODA_TEST_FAIL=", "DOCKER_HOST=", "DOCKER_CONTEXT=", "BUILDX_BUILDER=", "SODA_TEST_DOCKER_CASE=")
	return cmd
}

func runScriptOK(t *testing.T, cmd *exec.Cmd) string {
	t.Helper()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, output)
	}
	return string(output)
}

func runScriptFails(t *testing.T, cmd *exec.Cmd, message string) string {
	t.Helper()
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), message) {
		t.Fatalf("expected failure containing %q, got %v:\n%s", message, err, output)
	}
	return string(output)
}

func requireOrder(t *testing.T, commands string, expected ...string) {
	t.Helper()
	for _, command := range expected {
		index := strings.Index(commands, command)
		if index < 0 {
			t.Fatalf("missing or out-of-order %q:\n%s", command, commands)
		}
		commands = commands[index+len(command):]
	}
}

func writeStub(t *testing.T, bin, name, body string) {
	t.Helper()
	prefix := "#!/usr/bin/env bash\nset -eu\nprintf '%s' '" + name + "' >>\"$SODA_TEST_LOG\"; printf ' %s' \"$@\" >>\"$SODA_TEST_LOG\"; printf '\\n' >>\"$SODA_TEST_LOG\"\n"
	writeTestFile(t, filepath.Join(bin, name), prefix+body+"\n", 0o755)
}

func writeTestFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

// The subprocess checksum stub uses Go, not a platform-specific /usr/bin path.
func TestNativeChecksumHelper(t *testing.T) {
	if os.Getenv("SODA_TEST_BINARY") == "" {
		return
	}
	path := os.Args[len(os.Args)-1]
	contents, err := os.ReadFile(path)
	if err != nil {
		os.Exit(1)
	}
	if os.Getenv("SODA_TEST_FAIL") == "copy-checksum" && strings.Contains(path, "/destination/") {
		contents = []byte("different")
	}
	fmt.Printf("%x  %s\n", sha256.Sum256(contents), path)
	os.Exit(0)
}

const gitStub = `case "$*" in
'rev-parse --show-toplevel') echo "$SODA_TEST_ROOT" ;;
'status --porcelain=v1 --untracked-files=all')
  [[ ${SODA_TEST_FAIL:-} != git-status && ! -f "$SODA_TEST_ROOT/status-error" ]] || exit 1
  [[ ${SODA_TEST_FAIL:-} != dirty ]] || echo '?? source.go' ;;
'rev-parse HEAD'|'rev-parse origin/main')
  if [[ -f "$SODA_TEST_ROOT/drift" || ( "$*" == 'rev-parse origin/main' && ${SODA_TEST_FAIL:-} == origin ) ]]; then
    printf 'dddddddddddddddddddddddddddddddddddddddd\n'
  else echo '` + testRevision + `'; fi ;;
*) echo "unexpected git $*" >&2; exit 2 ;;
esac`

const dockerStub = `printf 'context=%s builder=%s\n' "${DOCKER_CONTEXT:-}" "${BUILDX_BUILDER:-}" >>"$SODA_TEST_LOG"
arch=$SODA_TEST_ARCH
case "${SODA_TEST_DOCKER_CASE:-}" in daemon) arch=wrong ;; esac
case "$*" in
'context show') echo native ;;
'context inspect native')
  if [[ ${SODA_TEST_DOCKER_CASE:-} == remote ]]; then echo '[{"Endpoints":{"docker":{"Host":"ssh://remote"}}}]'
  else echo '[{"Endpoints":{"docker":{"Host":"unix:///native.sock"}}}]'; fi ;;
'info --format {{json .}}')
  os=linux; [[ ${SODA_TEST_DOCKER_CASE:-} != daemon-os ]] || os=darwin
  printf '{"OSType":"%s","Architecture":"%s"}\n' "$os" "$arch" ;;
'buildx ls --format {{json .}}')
  if [[ ${SODA_TEST_DOCKER_CASE:-} == multiple ]]; then
    echo '{"Current":true,"Name":"native","Driver":"docker","Nodes":[{"Status":"running","Endpoint":"native"},{"Status":"running","Endpoint":"other"}]}'
    exit 0
  fi
  [[ ${SODA_TEST_DOCKER_CASE:-} != missing ]] || exit 0
  if [[ ${SODA_TEST_DOCKER_CASE:-} == unnamed ]]; then
    echo '{"Current":true,"Driver":"docker","Nodes":[{"Status":"running","Endpoint":"native"}]}'
    exit 0
  fi
  driver=docker; endpoint=native; status=running
  case "${SODA_TEST_DOCKER_CASE:-}" in driver) driver=docker-container ;; endpoint) endpoint=other ;; stopped) status=stopped ;; esac
  printf '{"Current":true,"Name":"native","Driver":"%s","Nodes":[{"Status":"%s","Endpoint":"%s","Platforms":["linux/amd64","linux/arm64"]}]}\n' "$driver" "$status" "$endpoint" ;;
'load --input '*) ;;
'image inspect --format {{.Id}} '*)
  if [[ ${SODA_TEST_FAIL:-} == image-id ]]; then echo wrong
  elif [[ ${SODA_TEST_IMAGE_STORE:-} == containerd ]]; then echo '` + testDigest + `'
  else echo '` + testImageID + `'; fi ;;
'build --platform '*) [[ ${SODA_TEST_FAIL:-} != check-image ]] ;;
'run --rm --pull=never --platform '*)
  [[ ${SODA_TEST_FAIL:-} != runtime && ${SODA_TEST_FAIL:-} != check-container ]] ;;
*) echo "unexpected docker $*" >&2; exit 2 ;;
esac`

const skopeoStub = `case "$*" in
'list-tags docker://ghcr.io/levitateos/soda-os')
  [[ ${SODA_TEST_FAIL:-} != tag-network ]] || exit 1
  if [[ ${SODA_TEST_FAIL:-} == tag ]]; then echo '{"Tags":["sha-` + testRevision + `-'"$SODA_TEST_ARCH"'"]}'; else echo '{"Tags":[]}'; fi ;;
'inspect oci-archive:'*)
  arch=arm64; [[ $SODA_TEST_ARCH != x86_64 ]] || arch=amd64
  [[ ${SODA_TEST_FAIL:-} != metadata ]] || arch=wrong
  printf '{"Os":"linux","Architecture":"%s","Digest":"` + testDigest + `","Labels":{"org.opencontainers.image.version":"0.6.3","org.opencontainers.image.revision":"` + testRevision + `"}}\n' "$arch" ;;
'inspect --raw oci-archive:'*)
  if [[ ${SODA_TEST_FAIL:-} == config-digest ]]; then echo '{"config":{"digest":"bad"}}'
  else echo '{"config":{"digest":"` + testImageID + `"}}'; fi ;;
'inspect --format {{.Digest}} oci-archive:'*)
  if [[ ${SODA_TEST_FAIL:-} == manifest-digest ]]; then echo bad; else echo '` + testDigest + `'; fi ;;
'inspect --no-creds --format {{.Digest}} docker://'* )
  [[ ${SODA_TEST_FAIL:-} != remote-network ]] || exit 1
  if [[ ${SODA_TEST_FAIL:-} == remote-digest ]]; then echo wrong; else echo '` + testDigest + `'; fi ;;
*) echo "unexpected skopeo $*" >&2; exit 2 ;;
esac`

const justStub = `case "$1" in
check)
  [[ ${SODA_TEST_FAIL:-} != check ]] || exit 1
  [[ ${SODA_TEST_FAIL:-} != drift-check ]] || touch "$SODA_TEST_ROOT/drift"
  [[ ${SODA_TEST_FAIL:-} != status-after-check ]] || touch "$SODA_TEST_ROOT/status-error" ;;
oci)
  [[ ${SODA_TEST_FAIL:-} != oci ]] || exit 1
  printf oci >".artifacts/images/soda-os-0.6.3-$2.oci.tar"
  [[ ${SODA_TEST_FAIL:-} != drift-oci ]] || touch "$SODA_TEST_ROOT/drift" ;;
iso)
  [[ ${SODA_TEST_FAIL:-} != iso ]] || exit 1
  iso=".artifacts/images/SodaOS-0.6.3-$2.iso"
  printf iso >"$iso"
  sum=$(sha256sum "$iso" | awk '{print $1}')
  [[ ${SODA_TEST_FAIL:-} != checksum ]] || sum=wrong
  printf '%s  %s\n' "$sum" "$(basename "$iso")" >"$iso.sha256"
  mkdir -p .artifacts/installer/context
  digest='` + testDigest + `'
  [[ ${SODA_TEST_FAIL:-} != binding ]] || digest=wrong
  printf 'bootc --source-imgref="docker://ghcr.io/levitateos/soda-os@%s" --target-imgref="ghcr.io/levitateos/soda-os@%s"\n' "$digest" "$digest" >.artifacts/installer/context/interactive-defaults.ks ;;
*) echo "unexpected just $*" >&2; exit 2 ;;
esac`

const goStub = `case "$*" in
'run ./cmd/soda-image --architecture '*' check') ;;
'run ./cmd/soda-release image-stage '*) [[ ${SODA_TEST_FAIL:-} != stage ]] ;;
*) echo "unexpected go $*" >&2; exit 2 ;;
esac`
