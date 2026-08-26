package observe

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func TestBrokerRefreshFilterSequenceAndCleanup(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	global := b.Subscribe(ctx, "")
	project := b.Subscribe(ctx, "p1")
	if message := receive(t, global.C); !message.Refresh {
		t.Fatal("first global message must be refresh")
	}
	if message := receive(t, project.C); !message.Refresh {
		t.Fatal("first project message must be refresh")
	}
	b.Publish(domain.EventGitChanged, "p1")
	first := receive(t, global.C)
	matching := receive(t, project.C)
	if first.Event == nil || matching.Event == nil || first.Event.Sequence != 1 || matching.Event.Sequence != 1 {
		t.Fatalf("expected matching sequence-one event, got %#v and %#v", first, matching)
	}
	b.Publish(domain.EventHostChanged, "")
	if event := receive(t, project.C).Event; event == nil || event.Sequence != 2 {
		t.Fatalf("global events should reach project subscriber: %#v", event)
	}
	project.Cancel()
	if _, open := <-project.C; open {
		t.Fatal("subscription channel should close on explicit cancellation")
	}
}

func TestBrokerCoalescesAndConvertsBackpressureToRefresh(t *testing.T) {
	b := NewBroker()
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscription := b.Subscribe(ctx, "")
	<-subscription.C
	b.Publish(domain.EventPeopleChanged, "")
	b.Publish(domain.EventPeopleChanged, "")
	if message := receive(t, subscription.C); message.Event == nil || message.Event.Sequence != 1 {
		t.Fatalf("expected one coalesced event, got %#v", message)
	}
	for i := 0; i < BrokerCapacity+8; i++ {
		project := "p" + strconv.Itoa(i)
		b.Publish(domain.EventGitChanged, project)
	}
	time.Sleep(CoalesceInterval + 100*time.Millisecond)
	foundRefresh := false
	deadline := time.After(2 * time.Second)
	for !foundRefresh {
		select {
		case message := <-subscription.C:
			foundRefresh = message.Refresh
		case <-deadline:
			t.Fatal("lagged subscriber did not receive refresh")
		}
	}
}

func receive(t *testing.T, channel <-chan StreamMessage) StreamMessage {
	t.Helper()
	select {
	case message, ok := <-channel:
		if !ok {
			t.Fatal("unexpected closed subscription")
		}
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return StreamMessage{}
	}
}
