package main

import "testing"

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
