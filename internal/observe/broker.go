// Package observe contains read-only host, Git, and SSH telemetry for sodad.
package observe

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

const (
	BrokerCapacity   = 256
	CoalesceInterval = 250 * time.Millisecond
)

// StreamMessage is deliberately transport-neutral. The daemon maps it to the
// protobuf stream, which keeps observers independent from generated code.
type StreamMessage struct {
	Event   *domain.Event
	Refresh bool
}

// Subscription is a bounded, cancelable view of broker events. A refresh tells
// the consumer to re-fetch current state; events are never replayed.
type Subscription struct {
	C      <-chan StreamMessage
	cancel func()
}

func (s *Subscription) Cancel() { s.cancel() }

type eventKey struct {
	kind      domain.EventKind
	projectID string
}

type subscriber struct {
	projectID string
	ch        chan StreamMessage
	mu        sync.Mutex
	closed    bool
}

// Broker provides bounded fan-out. It assigns sequence numbers only to emitted
// events, coalesces noisy duplicate notifications, and converts lag into a
// refresh instead of retaining history.
type Broker struct {
	mu          sync.Mutex
	pending     map[eventKey]struct{}
	subscribers map[*subscriber]struct{}
	queue       chan eventKey
	overflow    bool
	closed      bool
	sequence    atomic.Uint64
	done        chan struct{}
	closeOnce   sync.Once
}

func NewBroker() *Broker {
	b := &Broker{
		pending:     make(map[eventKey]struct{}),
		subscribers: make(map[*subscriber]struct{}),
		queue:       make(chan eventKey, BrokerCapacity),
		done:        make(chan struct{}),
	}
	go b.dispatch()
	return b
}

// Publish schedules an event. It never blocks callers; an overfull queue is
// represented by a later refresh, so consumers cannot mistake dropped events
// for unchanged state.
func (b *Broker) Publish(kind domain.EventKind, projectID string) {
	key := eventKey{kind: kind, projectID: projectID}
	b.mu.Lock()
	_, pending := b.pending[key]
	if b.closed || pending {
		b.mu.Unlock()
		return
	}
	b.pending[key] = struct{}{}
	b.mu.Unlock()

	time.AfterFunc(CoalesceInterval, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.pending, key)
		if b.closed {
			return
		}
		select {
		case b.queue <- key:
		default:
			b.overflow = true
		}
	})
}

// Subscribe sends an explicit initial refresh and removes itself when ctx is
// canceled. An empty projectID receives all events; a project subscription also
// receives global events.
func (b *Broker) Subscribe(ctx context.Context, projectID string) *Subscription {
	ch := make(chan StreamMessage, BrokerCapacity)
	s := &subscriber{projectID: projectID, ch: ch}
	ch <- StreamMessage{Refresh: true}

	b.mu.Lock()
	if b.closed {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(ch)
	} else {
		b.subscribers[s] = struct{}{}
	}
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.removeSubscriber(s)
		})
	}
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-b.done:
		}
	}()
	return &Subscription{C: ch, cancel: cancel}
}

func (b *Broker) removeSubscriber(s *subscriber) {
	b.mu.Lock()
	delete(b.subscribers, s)
	b.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

func (b *Broker) dispatch() {
	for {
		select {
		case key := <-b.queue:
			b.broadcast(StreamMessage{Event: &domain.Event{
				Kind:      key.kind,
				ProjectID: optionalProjectID(key.projectID),
				Sequence:  b.sequence.Add(1),
			}})
			b.mu.Lock()
			overflow := b.overflow
			b.overflow = false
			b.mu.Unlock()
			if overflow {
				b.broadcast(StreamMessage{Refresh: true})
			}
		case <-b.done:
			return
		}
	}
}

func (b *Broker) broadcast(message StreamMessage) {
	b.mu.Lock()
	subscribers := make([]*subscriber, 0, len(b.subscribers))
	for s := range b.subscribers {
		if message.Event == nil || s.projectID == "" || message.Event.ProjectID == nil || s.projectID == *message.Event.ProjectID {
			subscribers = append(subscribers, s)
		}
	}
	b.mu.Unlock()
	for _, s := range subscribers {
		s.deliver(message)
	}
}

func (s *subscriber) deliver(message StreamMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- message:
		return
	default:
	}
	// A receiver that falls behind receives a single resynchronization signal.
	for len(s.ch) > 0 {
		<-s.ch
	}
	select {
	case s.ch <- StreamMessage{Refresh: true}:
	default:
	}
}

func optionalProjectID(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (b *Broker) Close() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		subscribers := make([]*subscriber, 0, len(b.subscribers))
		for s := range b.subscribers {
			subscribers = append(subscribers, s)
		}
		b.subscribers = make(map[*subscriber]struct{})
		b.mu.Unlock()
		close(b.done)
		for _, s := range subscribers {
			s.mu.Lock()
			if !s.closed {
				s.closed = true
				close(s.ch)
			}
			s.mu.Unlock()
		}
	})
}
