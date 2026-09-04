package acceptance

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type CleanupAction struct {
	Name string
	Run  func(context.Context) error
}

type Cleanup struct {
	mu      sync.Mutex
	actions []CleanupAction
	done    bool
}

func (cleanup *Cleanup) Add(action CleanupAction) error {
	if action.Name == "" || action.Run == nil {
		return errors.New("cleanup action requires a name and operation")
	}
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	if cleanup.done {
		return errors.New("cleanup already ran")
	}
	cleanup.actions = append(cleanup.actions, action)
	return nil
}

func (cleanup *Cleanup) Run(ctx context.Context) error {
	cleanup.mu.Lock()
	if cleanup.done {
		cleanup.mu.Unlock()
		return nil
	}
	cleanup.done = true
	actions := append([]CleanupAction(nil), cleanup.actions...)
	cleanup.mu.Unlock()
	var joined error
	for index := len(actions) - 1; index >= 0; index-- {
		if err := actions[index].Run(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("cleanup %s: %w", actions[index].Name, err))
		}
	}
	return joined
}
