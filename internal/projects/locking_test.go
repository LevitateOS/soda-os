package projects

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/stretchr/testify/require"
)

func TestCoordinatorSetupHoldsSharedOperationLockThroughPublication(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	blocking := newBlockingTestPKExec(t)
	fixture.coordinator.privileged = blocking.invoker
	require.NoError(t, fixture.store.Add(catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
	}))

	setupResult := make(chan error, 1)
	go func() {
		_, err := fixture.coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site"}`))
		setupResult <- err
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(blocking.startedPath)
		return err == nil
	}, time.Second, 10*time.Millisecond, "setup did not reach publication")

	type lockResult struct {
		lock io.Closer
		err  error
	}
	exclusiveResult := make(chan lockResult, 1)
	go func() {
		lock, err := fixture.coordinator.operationLocks.Exclusive()
		exclusiveResult <- lockResult{lock: lock, err: err}
	}()
	requireBlocked(t, exclusiveResult, "exclusive removal lock acquired while setup was publishing")
	require.NoError(t, os.WriteFile(blocking.releasePath, nil, 0o600))
	require.NoError(t, requireResult(t, setupResult))
	acquired := requireResult(t, exclusiveResult)
	require.NoError(t, acquired.err)
	require.NoError(t, acquired.lock.Close())
}

type helperLockCase struct {
	name    string
	action  string
	request string
	prepare func(*testing.T, *helperFixture)
}

func TestHelperSetupStepsUseSharedOperationLock(t *testing.T) {
	tests := []helperLockCase{
		{name: "prepare", action: "workspace-prepare", request: `{"id":"site","canonical_url":"git@git.example.test:other.git"}`, prepare: addSite},
		{name: "publish", action: "workspace-publish", request: `{"id":"site","canonical_url":"git@git.example.test:other.git"}`, prepare: addSite},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHelperFixture(t)
			test.prepare(t, &fixture)
			held, err := fixture.helper.operationLocks.Exclusive()
			require.NoError(t, err)
			assertHelperActionBlocks(t, fixture, held, test, "project URL changed")
		})
	}
}

func TestHelperDestructiveActionsUseExclusiveOperationLock(t *testing.T) {
	tests := []helperLockCase{
		{name: "own removal", action: "workspace-remove", request: `{"id":"site"}`, prepare: addSite},
		{name: "project removal", action: "project-remove", request: `{"id":"site"}`, prepare: addSite},
		{name: "human deletion", action: "human-delete", request: `{"username":"target"}`, prepare: func(_ *testing.T, fixture *helperFixture) {
			fixture.host.accounts["target"] = rootPrimary("target")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHelperFixture(t)
			test.prepare(t, &fixture)
			held, err := fixture.helper.operationLocks.Shared()
			require.NoError(t, err)
			assertHelperActionBlocks(t, fixture, held, test, "")
		})
	}
}

func assertHelperActionBlocks(t *testing.T, fixture helperFixture, held io.Closer, test helperLockCase, expectedError string) {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		_, executeErr := fixture.helper.Execute(context.Background(), helperAlice(), test.action, strings.NewReader(test.request))
		result <- executeErr
	}()
	requireBlocked(t, result, "helper mutation returned without acquiring its operation lock")
	require.NoError(t, held.Close())
	executeErr := requireResult(t, result)
	if expectedError == "" {
		require.NoError(t, executeErr)
		return
	}
	require.ErrorContains(t, executeErr, expectedError)
}

func TestCatalogAddDoesNotAcquireWorkspaceOperationLock(t *testing.T) {
	fixture := newHelperFixture(t)
	held, err := fixture.helper.operationLocks.Exclusive()
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, executeErr := fixture.helper.Execute(context.Background(), helperAlice(), "catalog-add", strings.NewReader(
			`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git"}`,
		))
		result <- executeErr
	}()
	require.NoError(t, requireResult(t, result), "catalog add must use only the catalog lock")
	require.NoError(t, held.Close())
}

func addSite(t *testing.T, fixture *helperFixture) {
	t.Helper()
	require.NoError(t, fixture.store.Add(catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
	}))
}

func requireBlocked[T any](t *testing.T, result <-chan T, message string) {
	t.Helper()
	select {
	case <-result:
		require.FailNow(t, message)
	case <-time.After(50 * time.Millisecond):
	}
}

func requireResult[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(time.Second):
		require.FailNow(t, "operation did not complete")
		var zero T
		return zero
	}
}
