// Package updates exposes native bootc operations without retaining deployment state.
package updates

import (
	"context"

	"github.com/LevitateOS/soda-os/internal/process"
)

// Operations separates JSON queries from streamed native progress. Both runners
// are supplied by the caller; bootc owns source resolution and activation.
type Operations struct {
	Queries process.Runner
	Runner  process.Runner
}

func (o Operations) Status(ctx context.Context) (Host, error) {
	return ReadHost(ctx, o.Queries)
}

func (o Operations) Check(ctx context.Context) (Host, error) {
	if err := o.Runner.Run(ctx, process.Command{Name: "/usr/bin/bootc", Args: []string{"upgrade", "--check"}}); err != nil {
		return Host{}, err
	}
	return o.Status(ctx)
}

func (o Operations) Update(ctx context.Context) error {
	host, err := o.Status(ctx)
	if err != nil {
		return err
	}
	if err := host.mutable(); err != nil {
		return err
	}
	return o.Runner.Run(ctx, process.Command{Name: "/usr/bin/bootc", Args: []string{"upgrade", "--apply"}})
}
