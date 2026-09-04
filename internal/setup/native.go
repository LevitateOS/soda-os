package setup

import (
	"github.com/LevitateOS/soda-os/internal/projects"
	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func NewNativeService() Service {
	runner := projects.ExecCommandRunner{}
	platform := projects.NewNativePlatform()
	platform.Runner = runner
	return Service{
		Accounts: NativeAccounts{Platform: platform, Runner: runner},
		Forgejo:  NativeForgejo{Runner: runner},
		Network: NativeNetwork{
			Runner: runner, Tailnet: tailnet.New(tailnet.Options{}),
		},
		Completion: FileCompletion{},
		Locker:     FileLocker{},
	}
}
