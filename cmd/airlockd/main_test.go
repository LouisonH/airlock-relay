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

func TestRequireAllowedLANListen(t *testing.T) {
	for _, address := range []string{"0.0.0.0:4768", "192.168.1.10:4768", "10.0.0.5:4770"} {
		if err := requireAllowedListen(address, true); err != nil {
			t.Errorf("requireAllowedListen(%q) error = %v", address, err)
		}
	}
	for _, address := range []string{"203.0.113.10:4768", "example.com:4768"} {
		if err := requireAllowedListen(address, true); err == nil {
			t.Errorf("requireAllowedListen(%q) succeeded", address)
		}
	}
}

func TestAdvertisedAddressUsesLoopbackFallback(t *testing.T) {
	if got := advertisedAddressString("127.0.0.1:4770"); got != "127.0.0.1:4770" {
		t.Fatalf("advertised address = %q", got)
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
