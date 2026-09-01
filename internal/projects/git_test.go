package projects

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testCredentialUsername = "soda-auth-user-94e3d2"
	testCredentialPassword = "soda-auth-secret-7b51c8"
)

func TestMain(testingMain *testing.M) {
	if IsCredentialHelperInvocation() {
		os.Exit(runTestCredentialHelper())
	}
	os.Exit(testingMain.Run())
}

func runTestCredentialHelper() int {
	processSurface := strings.Join(append(append([]string{}, os.Args...), os.Environ()...), "\x00")
	if strings.Contains(processSurface, testCredentialUsername) || strings.Contains(processSurface, testCredentialPassword) {
		return 1
	}
	if err := captureTestCredentialHelper(); err != nil {
		return 1
	}
	if len(os.Args) != 2 {
		return 1
	}
	if err := RunCredentialHelper(os.Args[1], os.Stdin, os.Stdout); err != nil {
		return 1
	}
	return 0
}

func captureTestCredentialHelper() error {
	capture := os.Getenv("SODA_TEST_CREDENTIAL_HELPER_CAPTURE")
	if capture == "" {
		return nil
	}
	file, err := os.OpenFile(capture, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintf(file, "argv=%q\n", os.Args)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func TestGitClonePassesCredentialsThroughInheritedAnonymousDescriptor(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Git is not installed")
	}
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(testCredentialUsername+":"+testCredentialPassword))
	authenticated := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != expected {
			writer.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		authenticated = true
		writer.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("invalid Git advertisement"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "checkout")
	err := (GitCloner{Stdout: io.Discard, Stderr: io.Discard}).Clone(context.Background(), server.URL+"/team/site.git", destination, CloneCredentials{Username: testCredentialUsername, Password: testCredentialPassword})
	require.Error(t, err, "the intentionally invalid Git advertisement must fail the clone")
	require.True(t, authenticated, "real Git must invoke the credential helper through inherited fd 3")
}

func TestGitRejectsCredentialsForSSHRemote(t *testing.T) {
	err := (GitCloner{Stdout: io.Discard, Stderr: io.Discard}).Clone(context.Background(), "git@example.test:alice/site.git", filepath.Join(t.TempDir(), "checkout"), CloneCredentials{Username: "alice", Password: "secret"})
	require.ErrorContains(t, err, "only for HTTP")
}

func TestGitCloneDoesNotExposeCredentialInArgvEnvironmentOrRemote(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Git is not installed")
	}
	root := prepareDumbRepository(t)

	expectedAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(testCredentialUsername+":"+testCredentialPassword))
	fileServer := http.FileServer(http.Dir(root))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != expectedAuthorization {
			writer.Header().Set("WWW-Authenticate", `Basic realm="workspace proof"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		fileServer.ServeHTTP(writer, request)
	}))
	defer server.Close()

	remote := server.URL + "/remote.git"
	destination := filepath.Join(root, "checkout")
	helperCapture := filepath.Join(root, "credential-helper-process.txt")
	t.Setenv("SODA_TEST_CREDENTIAL_HELPER_CAPTURE", helperCapture)
	require.NoError(t, (GitCloner{Stdout: io.Discard, Stderr: io.Discard}).Clone(
		context.Background(), remote, destination,
		CloneCredentials{Username: testCredentialUsername, Password: testCredentialPassword},
	))
	config, err := os.ReadFile(filepath.Join(destination, ".git", "config"))
	require.NoError(t, err)
	require.Contains(t, string(config), remote)
	require.NotContains(t, string(config), testCredentialUsername)
	require.NotContains(t, string(config), testCredentialPassword)
	helperProcess, err := os.ReadFile(helperCapture)
	require.NoError(t, err)
	require.NotContains(t, string(helperProcess), testCredentialUsername)
	require.NotContains(t, string(helperProcess), testCredentialPassword)
}

func TestGitClonesPublicHTTPWithoutCredentials(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Git is not installed")
	}
	root := prepareDumbRepository(t)
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()

	destination := filepath.Join(root, "public-checkout")
	require.NoError(t, (GitCloner{Stdout: io.Discard, Stderr: io.Discard}).Clone(
		context.Background(), server.URL+"/remote.git", destination, CloneCredentials{},
	))
	contents, err := os.ReadFile(filepath.Join(destination, "README.md"))
	require.NoError(t, err)
	require.Equal(t, "workspace proof\n", string(contents))
}

func TestGitChildReceivesNoCredentialBearingArgumentOrEnvironmentValue(t *testing.T) {
	root := t.TempDir()
	capture := filepath.Join(root, "capture")
	t.Setenv("SODA_TEST_GIT_CAPTURE", capture)
	t.Setenv("GIT_CONFIG_PARAMETERS", "'credential.helper=evil-helper'")
	t.Setenv("GIT_TRACE_CURL", "1")
	t.Setenv("GIT_TRACE_REDACT", "0")
	t.Setenv("GIT_CURL_VERBOSE", "1")
	t.Setenv("HTTP_PROXY", "http://proxy.example.test:8080")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.test:8080")
	t.Setenv("ALL_PROXY", "socks5://proxy.example.test:1080")
	t.Setenv("SSLKEYLOGFILE", filepath.Join(root, "tls-keys.log"))
	probe := filepath.Join(root, "git-probe")
	require.NoError(t, os.WriteFile(probe, []byte(`#!/bin/sh
tr '\0' '\n' </proc/self/cmdline >"${SODA_TEST_GIT_CAPTURE}.argv"
/usr/bin/env >"${SODA_TEST_GIT_CAPTURE}.env"
exit 1
`), 0o700))

	err := (GitCloner{Binary: probe, Stdout: io.Discard, Stderr: io.Discard}).Clone(
		context.Background(), "https://git.example.test/team/site.git", filepath.Join(root, "checkout"),
		CloneCredentials{Username: "auth-user", Password: "one-use-secret"},
	)
	require.Error(t, err)
	for _, suffix := range []string{".argv", ".env"} {
		contents, readErr := os.ReadFile(capture + suffix)
		require.NoError(t, readErr)
		require.False(t, bytes.Contains(contents, []byte("auth-user")), suffix)
		require.False(t, bytes.Contains(contents, []byte("one-use-secret")), suffix)
		if suffix == ".env" {
			for _, forbidden := range []string{
				"GIT_CONFIG_PARAMETERS=", "GIT_TRACE_CURL=", "GIT_TRACE_REDACT=",
				"GIT_CURL_VERBOSE=", "SSLKEYLOGFILE=",
			} {
				require.NotContains(t, string(contents), forbidden)
			}
			require.Contains(t, string(contents), "HTTP_PROXY=http://proxy.example.test:8080")
			require.Contains(t, string(contents), "HTTPS_PROXY=http://proxy.example.test:8080")
			require.Contains(t, string(contents), "ALL_PROXY=socks5://proxy.example.test:1080")
			require.Contains(t, string(contents), "GIT_CONFIG_COUNT=3")
			require.Contains(t, string(contents), "GIT_CONFIG_KEY_2=credential.interactive")
			require.Contains(t, string(contents), "GIT_CONFIG_VALUE_2=false")
		}
	}
}

func TestGitCredentialHelperRejectsCrossAuthorityRedirect(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Git is not installed")
	}
	root := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	globalConfig := filepath.Join(root, "gitconfig")
	runGit(t, root, "config", "--file", globalConfig, "core.askPass", executable)
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	helperCapture := filepath.Join(root, "credential-helper-process.txt")
	t.Setenv("SODA_TEST_CREDENTIAL_HELPER_CAPTURE", helperCapture)

	var redirectedAuthorization string
	redirected := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		redirectedAuthorization = request.Header.Get("Authorization")
		writer.Header().Set("WWW-Authenticate", `Basic realm="other host"`)
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer redirected.Close()
	original := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, redirected.URL+request.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	defer original.Close()

	err = (GitCloner{Stdout: io.Discard, Stderr: io.Discard}).Clone(
		context.Background(), original.URL+"/team/site.git", filepath.Join(root, "checkout"),
		CloneCredentials{Username: testCredentialUsername, Password: testCredentialPassword},
	)
	require.Error(t, err)
	require.Empty(t, redirectedAuthorization, "credentials bound to the original authority must not cross a redirect")
	helperProcess, err := os.ReadFile(helperCapture)
	require.NoError(t, err)
	require.NotContains(t, string(helperProcess), "Username for", "ambient core.askPass must not run")
	require.NotContains(t, string(helperProcess), "Password for", "ambient core.askPass must not run")
}

func TestGitCredentialHelperStoreAndEraseAreNoOps(t *testing.T) {
	for _, operation := range []string{"store", "erase"} {
		t.Run(operation, func(t *testing.T) {
			input := strings.NewReader("username=alice\npassword=one-use-secret\n\n")
			remaining := input.Len()
			var output bytes.Buffer

			require.NoError(t, RunCredentialHelper(operation, input, &output))
			require.Equal(t, remaining, input.Len(), "a no-op must not consume the supplied credential")
			require.Empty(t, output.String())
		})
	}
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("/usr/bin/git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s: %s", strings.Join(command.Args, " "), output)
}

func prepareDumbRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "remote.git")
	work := filepath.Join(root, "work")
	runGit(t, root, "init", "--bare", bare)
	require.NoError(t, os.Mkdir(work, 0o700))
	runGit(t, work, "init")
	runGit(t, work, "config", "user.name", "Test User")
	runGit(t, work, "config", "user.email", "test@example.invalid")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("workspace proof\n"), 0o644))
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "proof")
	runGit(t, work, "push", bare, "HEAD:refs/heads/main")
	runGit(t, root, "--git-dir="+bare, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, root, "--git-dir="+bare, "update-server-info")
	return root
}
