package cli

import (
	"strings"
	"testing"
)

func TestCheckBinaryReportsCurrentExecutable(t *testing.T) {
	got := checkBinary()

	if got.Name != "binary" {
		t.Fatalf("Name = %q, want binary", got.Name)
	}
	if got.Status != "ok" {
		t.Fatalf("Status = %q, want ok", got.Status)
	}
	if !strings.Contains(got.Message, "Binary running from:") {
		t.Fatalf("Message = %q, want current executable path", got.Message)
	}
	if strings.Contains(got.Message, "PATH") {
		t.Fatalf("Message = %q, want no PATH lookup wording", got.Message)
	}
}
