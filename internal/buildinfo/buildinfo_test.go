package buildinfo

import "testing"

func TestVersionDefaults(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must never be empty")
	}
	if Commit == "" {
		t.Fatal("Commit must never be empty")
	}
}
