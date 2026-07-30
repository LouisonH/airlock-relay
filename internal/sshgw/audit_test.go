package sshgw

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandAuditRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh-command-audit.json")
	audit, err := OpenFileCommandAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	event := CommandEvent{
		Time: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC), RouteAlias: "nas",
		Command: "uname -a", Result: "allowed", DurationMS: 12, Egress: "Auto",
	}
	if err := audit.Record(event); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("command audit permissions = %o", info.Mode().Perm())
	}
	loaded, err := OpenFileCommandAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	events := loaded.List(10)
	if len(events) != 1 || events[0].RouteAlias != "nas" || events[0].Command != "uname -a" || events[0].ID == "" {
		t.Fatalf("command audit events = %+v", events)
	}
	if err := loaded.Record(CommandEvent{RouteAlias: "nas", Command: "whoami\ncat /etc/passwd", Result: "blocked"}); err == nil {
		t.Fatal("command audit accepted a command containing a newline")
	}
}
