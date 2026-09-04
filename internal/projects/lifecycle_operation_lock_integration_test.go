package projects

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type realOperationLockPlatform struct {
	*fakePlatform
	path                string
	exclusiveAttempts   chan struct{}
	raceSynchronization sync.RWMutex
}

func (platform *realOperationLockPlatform) WorkspaceOperationSharedLock() (io.Closer, error) {
	lock, err := openWorkspaceOperationLock(platform.path, unix.Getuid(), unix.LOCK_SH)
	if err != nil {
		return nil, err
	}
	platform.raceSynchronization.RLock()
	return synchronizedOperationLock{Closer: lock, unlock: platform.raceSynchronization.RUnlock}, nil
}

func (platform *realOperationLockPlatform) WorkspaceOperationExclusiveLock() (io.Closer, error) {
	platform.exclusiveAttempts <- struct{}{}
	lock, err := openWorkspaceOperationLock(platform.path, unix.Getuid(), unix.LOCK_EX)
	if err != nil {
		return nil, err
	}
	platform.raceSynchronization.Lock()
	return synchronizedOperationLock{Closer: lock, unlock: platform.raceSynchronization.Unlock}, nil
}

type synchronizedOperationLock struct {
	io.Closer
	unlock func()
}

func (lock synchronizedOperationLock) Close() error {
	lock.unlock()
	return lock.Closer.Close()
}

func newWorkspaceForgejoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/user":
			_, _ = writer.Write([]byte(`{"login":"alice"}`))
		case "/api/v1/user/keys":
			if request.Method == http.MethodGet {
				_, _ = writer.Write([]byte(`[]`))
				return
			}
			writer.WriteHeader(http.StatusCreated)
		}
	}))
}

func TestSetupOperationLockBlocksProjectAndHumanRemoval(t *testing.T) {
	catalog := testCatalog(t)
	require.NoError(t, catalog.Add(CatalogEntry{
		ID:           "site",
		DisplayName:  "Site",
		CanonicalURL: "git@git.example.test:site.git",
	}))

	basePlatform := newFakePlatform()
	basePlatform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	basePlatform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform := &realOperationLockPlatform{
		fakePlatform:      basePlatform,
		path:              testOperationLockFile(t),
		exclusiveAttempts: make(chan struct{}, 2),
	}
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	publishStarted := make(chan struct{})
	releasePublish := make(chan struct{})
	released := false
	t.Cleanup(func() {
		if !released {
			close(releasePublish)
		}
	})
	privileged := &fakePrivileged{
		workspacePublicKey: strings.TrimSpace(string(testAuthorizedKey(t))),
		publishStarted:     publishStarted, publishRelease: releasePublish,
	}
	forgejo := newWorkspaceForgejoServer(t)
	defer forgejo.Close()
	coordinator := Coordinator{
		Catalog:       catalog,
		Lifecycle:     lifecycle,
		Platform:      platform,
		Privileged:    privileged,
		Forgejo:       ForgejoClient{},
		Tailnet:       &fakeTailnetIdentity{},
		Hostname:      "soda",
		ForgejoAPIURL: forgejo.URL,
	}

	setupResult := make(chan error, 1)
	go func() {
		_, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","forgejo_password":"one-use"}`))
		setupResult <- err
	}()
	requireSignal(t, publishStarted, "setup did not reach clone publication while holding the shared operation lock")

	removeProjectResult := make(chan error, 1)
	go func() {
		removeProjectResult <- lifecycle.RemoveProject(context.Background(), "admin", "site")
	}()
	deleteHumanResult := make(chan error, 1)
	go func() {
		deleteHumanResult <- lifecycle.DeleteHuman(context.Background(), "admin", "alice")
	}()
	requireSignal(t, platform.exclusiveAttempts, "project removal did not attempt the exclusive operation lock")
	requireSignal(t, platform.exclusiveAttempts, "human deletion did not attempt the exclusive operation lock")
	requireOperationBlocked(t, removeProjectResult, "project removal returned while setup held the shared operation lock")
	requireOperationBlocked(t, deleteHumanResult, "human deletion returned while setup held the shared operation lock")

	close(releasePublish)
	released = true
	requireOperationResult(t, setupResult)
	requireOperationResult(t, removeProjectResult)
	requireOperationResult(t, deleteHumanResult)
}

func requireSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		require.FailNow(t, message)
	}
}

func requireOperationBlocked(t *testing.T, result <-chan error, message string) {
	t.Helper()
	select {
	case err := <-result:
		require.Fail(t, message, "%v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func requireOperationResult(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "operation did not complete after the setup lock was released")
	}
}
