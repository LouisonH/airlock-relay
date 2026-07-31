package securefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRejectsUnsafeFiles(t *testing.T) {
	directory := t.TempDir()
	unsafe := filepath.Join(directory, "unsafe.json")
	if err := os.WriteFile(unsafe, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(unsafe, 1024); err == nil {
		t.Fatal("Read accepted a group-readable file")
	}

	safe := filepath.Join(directory, "safe.json")
	if err := os.WriteFile(safe, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	contents, err := Read(safe, 1024)
	if err != nil || string(contents) != "{}" {
		t.Fatalf("Read() = %q, %v", contents, err)
	}
}

func TestCreateAndReadToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.token")
	created, err := CreateToken(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadToken(path)
	if err != nil || loaded != created {
		t.Fatalf("ReadToken() = %q, %v", loaded, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("token permissions = %v, %v", info.Mode().Perm(), err)
	}
	if _, err := CreateToken(path); err == nil {
		t.Fatal("CreateToken replaced an existing token")
	}
}
