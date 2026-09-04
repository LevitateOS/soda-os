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
		Accounts: NativeAccounts{Host: host},
		Network: NativeNetwork{
			Runner: runner, Tailnet: tailnet.New(tailnet.Options{}),
		},
		Locker:     FileLocker{},
		Completion: FileCompletion{},
	}
}
