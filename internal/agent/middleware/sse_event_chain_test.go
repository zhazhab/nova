package middleware

import (
	"strings"
	"testing"

	"nova/internal/agent"
)

func TestSSEEventMiddlewareChainRunsMiddlewaresInOrder(t *testing.T) {
	var calls []string
	collector := &sseEventCollector{}
	chain := newSSEEventMiddlewareChainWithMiddlewares(
		&sseRecordingMiddleware{name: "first", calls: &calls, nextType: "first_seen"},
		&sseRecordingMiddleware{name: "second", calls: &calls, nextType: "second_seen"},
	)

	if err := chain.Next(collector.Handle)(agent.Event{Type: "start", Data: nil}); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if len(collector.events) != 1 {
		t.Fatalf("forwarded events = %d, want 1", len(collector.events))
	}
	got := collector.events[0]
	if got.Type != "second_seen" {
		t.Fatalf("event type = %q, want second_seen", got.Type)
	}
	if strings.Join(calls, ",") != "first,second" {
		t.Fatalf("middleware order = %v", calls)
	}
	if got.Data != nil {
		t.Fatalf("nil event data should remain forwardable, got %#v", got.Data)
	}
}

func TestSSEEventMiddlewareChainStopsWhenMiddlewareDoesNotCallNext(t *testing.T) {
	var calls []string
	collector := &sseEventCollector{}
	chain := newSSEEventMiddlewareChainWithMiddlewares(
		&sseRecordingMiddleware{name: "first", calls: &calls, suppress: true},
		&sseRecordingMiddleware{name: "second", calls: &calls},
	)

	if err := chain.Next(collector.Handle)(agent.Event{Type: "start", Data: map[string]string{}}); err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	if strings.Join(calls, ",") != "first" {
		t.Fatalf("middleware calls = %v, want only first", calls)
	}
	if len(collector.events) != 0 {
		t.Fatalf("forwarded events = %d, want 0", len(collector.events))
	}
}

type sseRecordingMiddleware struct {
	name     string
	calls    *[]string
	nextType string
	suppress bool
}

func (m *sseRecordingMiddleware) Next(next SSEEventHandler) SSEEventHandler {
	return func(ev agent.Event) error {
		*m.calls = append(*m.calls, m.name)
		if m.suppress {
			return nil
		}
		if m.nextType != "" {
			ev.Type = m.nextType
		}
		return next(ev)
	}
}
