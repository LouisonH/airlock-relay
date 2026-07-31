package activity

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const maxEvents = 200

type Event struct {
	ID         string
	Time       time.Time
	RouteAlias string
	Category   string
	EventType  string
	Caller     string
	Action     string
	Result     string
	DurationMS int64
	Egress     string
}

type Recorder interface {
	Record(Event) error
	List(limit int) []Event
}

type MemoryRecorder struct {
	mu     sync.Mutex
	events []Event
}

func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{}
}

func (r *MemoryRecorder) Record(event Event) error {
	if !validEvent(event) {
		return errors.New("invalid sanitized activity event")
	}
	if event.ID == "" {
		event.ID = newEventID()
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	if len(r.events) > maxEvents {
		r.events = append([]Event(nil), r.events[len(r.events)-maxEvents:]...)
	}
	return nil
}

func (r *MemoryRecorder) List(limit int) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > len(r.events) {
		limit = len(r.events)
	}
	result := make([]Event, 0, limit)
	for index := len(r.events) - 1; index >= len(r.events)-limit; index-- {
		result = append(result, r.events[index])
	}
	return result
}

func validEvent(event Event) bool {
	if event.RouteAlias == "" || len(event.RouteAlias) > 63 || event.Caller == "" || len(event.Caller) > 80 || strings.TrimSpace(event.Action) == "" || len(event.Action) > 160 || event.DurationMS < 0 {
		return false
	}
	switch event.Category {
	case "HTTP", "SSH", "LLM", "System":
	default:
		return false
	}
	switch event.EventType {
	case "request", "command", "health":
	default:
		return false
	}
	if event.Result != "allowed" && event.Result != "blocked" && event.Result != "failed" {
		return false
	}
	return event.Egress == "" || event.Egress == "Direct" || event.Egress == "Proxy" || event.Egress == "Auto"
}

func newEventID() string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err == nil {
		return "event-" + hex.EncodeToString(random)
	}
	return "event-" + time.Now().UTC().Format("20060102T150405.000000000")
}
