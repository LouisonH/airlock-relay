package activity

import (
	"testing"
	"time"
)

func TestMemoryRecorderReturnsNewestSanitizedEventsFirst(t *testing.T) {
	recorder := NewMemoryRecorder()
	for _, event := range []Event{
		{Time: time.Unix(1, 0), RouteAlias: "downloads", Category: "HTTP", EventType: "request", Caller: "loopback", Action: "GET request", Result: "allowed", Egress: "Direct"},
		{Time: time.Unix(2, 0), RouteAlias: "coding", Category: "LLM", EventType: "health", Caller: "Airlock Desktop", Action: "Manual health check", Result: "failed", DurationMS: 12, Egress: "Auto"},
	} {
		if err := recorder.Record(event); err != nil {
			t.Fatal(err)
		}
	}
	events := recorder.List(1)
	if len(events) != 1 || events[0].RouteAlias != "coding" || events[0].ID == "" {
		t.Fatalf("events = %+v", events)
	}
}

func TestMemoryRecorderRejectsSensitiveShapeExpansion(t *testing.T) {
	recorder := NewMemoryRecorder()
	if err := recorder.Record(Event{RouteAlias: "route", Category: "HTTP", EventType: "request", Caller: "loopback", Action: "", Result: "allowed"}); err == nil {
		t.Fatal("empty action was accepted")
	}
	if err := recorder.Record(Event{RouteAlias: "route", Category: "Credentials", EventType: "request", Caller: "loopback", Action: "GET request", Result: "allowed"}); err == nil {
		t.Fatal("unknown category was accepted")
	}
}
