package runners

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelperListRequiresAdministratorAndStrictEmptyRequest(t *testing.T) {
	helper := Helper{Authorizer: fakeAuthorizer{err: errors.New("not an administrator")}}
	_, err := helper.Execute(context.Background(), testAdministrator, "list", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "not an administrator")
	helper.Authorizer = fakeAuthorizer{}
	_, err = helper.Execute(context.Background(), testAdministrator, "list", strings.NewReader(`{"path":"/tmp/other-client"}`))
	require.ErrorContains(t, err, "unknown field")
	views := []RunnerView{{Descriptor: Descriptor{ID: "one"}, Version: "2.337.0"}}
	helper.Local = fakeLocal{views: views}
	response, err := helper.Execute(context.Background(), testAdministrator, "list", strings.NewReader(`{}`))
	require.NoError(t, err)
	require.Equal(t, views, response)
}

func TestInvokerDecodesPrivilegedRunnerList(t *testing.T) {
	views := []RunnerView{{Descriptor: Descriptor{ID: "one"}, Version: "2.337.0"}}
	body, err := json.Marshal(views)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "pkexec")
	script := "#!/bin/sh\n[ \"$1\" = --disable-internal-agent ] && [ \"$2\" = /runner-helper ] && [ \"$3\" = list ] || exit 2\n[ \"$(cat)\" = '{}' ] || exit 3\ncat <<'RESULT'\n" + string(body) + "\nRESULT\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	actual, err := (PKExecInvoker{Binary: path, HelperPath: "/runner-helper"}).List(context.Background())
	require.NoError(t, err)
	require.Equal(t, views, actual)
}
