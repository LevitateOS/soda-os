package scripts

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	for _, script := range []string{"check-native.sh", "prepare-native-image.sh", "place-libvirt-iso.sh"} {
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
'rev-parse HEAD')
  if [[ -f "$SODA_TEST_ROOT/drift" ]]; then
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
'inspect oci-archive:'*)
  arch=arm64; [[ $SODA_TEST_ARCH != x86_64 ]] || arch=amd64
  [[ ${SODA_TEST_FAIL:-} != metadata ]] || arch=wrong
  version=0.6.3; [[ ${SODA_TEST_FAIL:-} != version ]] || version=wrong
  printf '{"Os":"linux","Architecture":"%s","Digest":"` + testDigest + `","Labels":{"org.opencontainers.image.version":"%s","org.opencontainers.image.revision":"` + testRevision + `"}}\n' "$arch" "$version" ;;
'inspect --raw oci-archive:'*)
  if [[ ${SODA_TEST_FAIL:-} == config-digest ]]; then echo '{"config":{"digest":"bad"}}'
  else echo '{"config":{"digest":"` + testImageID + `"}}'; fi ;;
'inspect --format {{.Digest}} oci-archive:'*)
  if [[ ${SODA_TEST_FAIL:-} == manifest-digest ]]; then echo bad; else echo '` + testDigest + `'; fi ;;
*) echo "unexpected skopeo $*" >&2; exit 2 ;;
esac`

const justStub = `case "$1" in
check)
  [[ ${SODA_TEST_FAIL:-} != check ]] || exit 1
  [[ ${SODA_TEST_FAIL:-} != drift-check ]] || touch "$SODA_TEST_ROOT/drift"
  [[ ${SODA_TEST_FAIL:-} != status-after-check ]] || touch "$SODA_TEST_ROOT/status-error" ;;
forgejo-source|github-runner|mise-rpm|tea-source|cosign-source)
  [[ ${SODA_TEST_FAIL:-} != fetch ]] || exit 1 ;;
*) echo "unexpected just $*" >&2; exit 2 ;;
esac`

const goStub = `case "$*" in
'run ./cmd/soda-image --architecture '*' oci --output-dir '*)
  [[ ${SODA_TEST_FAIL:-} != oci ]] || exit 1
  archive="$SODA_TEST_ROOT/.artifacts/images/actual returned archive.oci.tar"
  [[ ${SODA_TEST_FAIL:-} == missing-archive ]] || printf oci >"$archive"
  printf 'build progress\n' >&2
  printf '%s\n' "$archive"
  [[ ${SODA_TEST_FAIL:-} != drift-oci ]] || touch "$SODA_TEST_ROOT/drift" ;;
'run ./cmd/soda-image --architecture '*' publish --archive '*)
  [[ "$*" == *'/actual returned archive.oci.tar' ]] || exit 2
  [[ ${SODA_TEST_FAIL:-} != publish ]] || { echo 'development tag may have advanced' >&2; exit 1; } ;;
*) echo "unexpected go $*" >&2; exit 2 ;;
esac`
