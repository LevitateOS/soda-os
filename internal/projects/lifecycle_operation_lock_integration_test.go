package projects

import (
	"context"
	"io"
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

type blockingSetupCloner struct {
	started chan struct{}
	release <-chan struct{}
}

func (cloner blockingSetupCloner) Clone(context.Context, string, string, CloneCredentials) error {
	close(cloner.started)
	<-cloner.release
	return nil
}

func TestSetupOperationLockBlocksProjectAndHumanRemoval(t *testing.T) {
	catalog := testCatalog(t)
	require.NoError(t, catalog.Add(CatalogEntry{
		ID:           "site",
		DisplayName:  "Site",
		CanonicalURL: "https://git.example.test/site.git",
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

	cloneStarted := make(chan struct{})
	releaseClone := make(chan struct{})
	released := false
	t.Cleanup(func() {
		if !released {
			close(releaseClone)
		}
	})
	coordinator := Coordinator{
		Catalog:    catalog,
		Lifecycle:  lifecycle,
		Platform:   platform,
		Privileged: &fakePrivileged{},
		Cloner:     blockingSetupCloner{started: cloneStarted, release: releaseClone},
		Endpoints:  fakeEndpoints{},
		Tea:        &fakeTea{},
	}

	setupResult := make(chan error, 1)
	go func() {
		_, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","git_username":"","git_password":""}`))
		setupResult <- err
	}()
	requireSignal(t, cloneStarted, "setup did not reach the clone while holding the shared operation lock")

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

	close(releaseClone)
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
