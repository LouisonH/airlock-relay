package sshgw

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/LouisonH/airlock-relay/internal/securefs"
)

const (
	commandAuditVersion   = 1
	maxCommandAuditBytes  = 1 << 20
	maxCommandAuditEvents = 100
)

type CommandEvent struct {
	ID         string    `json:"id"`
	Time       time.Time `json:"time"`
	RouteAlias string    `json:"route_alias"`
	Command    string    `json:"command"`
	Result     string    `json:"result"`
	DurationMS int64     `json:"duration_ms"`
	Egress     string    `json:"egress"`
}

type CommandAudit interface {
	Record(CommandEvent) error
	List(limit int) []CommandEvent
}

type FileCommandAudit struct {
	mu     sync.Mutex
	path   string
	events []CommandEvent
}

type commandAuditDocument struct {
	Version int            `json:"version"`
	Events  []CommandEvent `json:"events"`
}

func OpenFileCommandAudit(path string) (*FileCommandAudit, error) {
	store := &FileCommandAudit{path: path}
	if err := secureMetadataDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil || !securefs.IsPrivateRegularFile(info) || info.Size() > maxCommandAuditBytes {
		return nil, errors.New("invalid SSH command audit file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("open SSH command audit file")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("SSH command audit file changed while opening")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxCommandAuditBytes+1))
	decoder.DisallowUnknownFields()
	var document commandAuditDocument
	if err := decoder.Decode(&document); err != nil || document.Version != commandAuditVersion || len(document.Events) > maxCommandAuditEvents {
		return nil, errors.New("decode SSH command audit file")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("SSH command audit file has trailing data")
	}
	for _, event := range document.Events {
		if !validCommandEvent(event) {
			return nil, errors.New("invalid SSH command audit event")
		}
	}
	store.events = append([]CommandEvent(nil), document.Events...)
	return store, nil
}

func (s *FileCommandAudit) Record(event CommandEvent) error {
	if !validCommand(event.Command) || event.RouteAlias == "" || len(event.RouteAlias) > 63 {
		return errors.New("invalid SSH command audit event")
	}
	if event.ID == "" {
		event.ID = newCommandEventID()
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if !validCommandEvent(event) {
		return errors.New("invalid SSH command audit event")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	updated := append(append([]CommandEvent(nil), s.events...), event)
	if len(updated) > maxCommandAuditEvents {
		updated = append([]CommandEvent(nil), updated[len(updated)-maxCommandAuditEvents:]...)
	}
	if err := s.save(updated); err != nil {
		return err
	}
	s.events = updated
	return nil
}

func (s *FileCommandAudit) List(limit int) []CommandEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}
	result := make([]CommandEvent, 0, limit)
	for index := len(s.events) - 1; index >= len(s.events)-limit; index-- {
		result = append(result, s.events[index])
	}
	return result
}

func (s *FileCommandAudit) save(events []CommandEvent) error {
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".ssh-command-audit-*.tmp")
	if err != nil {
		return errors.New("create SSH command audit file")
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := securefs.PreparePrivateFile(temporary); err != nil {
		temporary.Close()
		return errors.New("protect SSH command audit file")
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(commandAuditDocument{Version: commandAuditVersion, Events: events}); err != nil {
		temporary.Close()
		return errors.New("encode SSH command audit file")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return errors.New("sync SSH command audit file")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close SSH command audit file")
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return errors.New("install SSH command audit file")
	}
	if err := securefs.SyncDirectory(filepath.Dir(s.path)); err != nil {
		return errors.New("sync SSH command audit directory")
	}
	return nil
}

func validCommandEvent(event CommandEvent) bool {
	if event.ID == "" || len(event.ID) > 64 || !validCommand(event.Command) || event.Time.IsZero() || event.RouteAlias == "" || len(event.RouteAlias) > 63 || event.DurationMS < 0 {
		return false
	}
	if event.Result != "allowed" && event.Result != "blocked" && event.Result != "failed" {
		return false
	}
	return event.Egress == "" || event.Egress == "Direct" || event.Egress == "Proxy" || event.Egress == "Auto"
}

func newCommandEventID() string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err == nil {
		return "ssh-" + hex.EncodeToString(random)
	}
	return "ssh-" + time.Now().UTC().Format("20060102T150405.000000000")
}
