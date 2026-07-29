package capability

import "testing"

func TestGenerateAndVerify(t *testing.T) {
	token, digest, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if token == "" {
		t.Fatal("Generate() returned an empty token")
	}
	if err := Verify(token, digest); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := Verify(token+"x", digest); err == nil {
		t.Fatal("Verify() accepted a modified token")
	}
}
