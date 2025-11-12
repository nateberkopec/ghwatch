package ghwatch

import (
	"os"
	"os/exec"
	"testing"
)

// TestAllPackages ensures `go test` (with no args) runs the full suite by
// delegating to `go test ./...` once. A guard env var prevents infinite loops.
func TestAllPackages(t *testing.T) {
	if os.Getenv("GHWATCH_ALL_TESTS") == "1" {
		return
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Env = append(os.Environ(), "GHWATCH_ALL_TESTS=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go test ./... failed: %v", err)
	}
}
