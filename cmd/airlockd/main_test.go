package main

import (
	"strings"
	"testing"
)

func TestRequireLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:4768", "[::1]:4768"} {
		if err := requireLoopback(address); err != nil {
			t.Errorf("requireLoopback(%q) error = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:4768", "192.168.1.10:4768", "localhost:4768"} {
		if err := requireLoopback(address); err == nil {
			t.Errorf("requireLoopback(%q) succeeded", address)
		}
	}
}

func TestReadControlToken(t *testing.T) {
	want := "airlock_control_ephemeral_token_32_bytes"
	got, err := readControlToken(strings.NewReader(want + "\n"))
	if err != nil {
		t.Fatalf("readControlToken() error = %v", err)
	}
	if got != want {
		t.Fatalf("readControlToken() = %q", got)
	}
	if _, err := readControlToken(strings.NewReader("too-short\n")); err == nil {
		t.Fatal("readControlToken() accepted a short token")
	}
}
