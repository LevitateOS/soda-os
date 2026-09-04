package setup

import (
	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func NewNativeService() Service {
	runner := linuxhost.ExecCommandRunner{}
	host := linuxhost.NewNative()
	host.Runner = runner
	return Service{
		Accounts: NativeAccounts{Host: host, Runner: runner},
		Forgejo:  NativeForgejo{Runner: runner},
		Network: NativeNetwork{
			Runner: runner, Tailnet: tailnet.New(tailnet.Options{}),
		},
		Completion: FileCompletion{},
		Locker:     FileLocker{},
	}
}
