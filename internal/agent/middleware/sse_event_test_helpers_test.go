package middleware

import (
	"testing"

	"nova/internal/agent"
)

type sseEventCollector struct {
	events []agent.Event
}

func (c *sseEventCollector) Handle(ev agent.Event) error {
	c.events = append(c.events, ev)
	return nil
}

func mustForwardSSEEvent(t *testing.T, collector *sseEventCollector, handler SSEEventHandler, ev agent.Event) agent.Event {
	t.Helper()
	before := len(collector.events)
	if err := handler(ev); err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	if len(collector.events) != before+1 {
		t.Fatalf("event should be forwarded: %#v", ev)
	}
	return collector.events[len(collector.events)-1]
}

func mustSuppressSSEEvent(t *testing.T, collector *sseEventCollector, handler SSEEventHandler, ev agent.Event) {
	t.Helper()
	before := len(collector.events)
	if err := handler(ev); err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	if len(collector.events) != before {
		t.Fatalf("event should be suppressed: %#v", collector.events[len(collector.events)-1])
	}
}
