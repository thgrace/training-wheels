package cli

import "testing"

func TestUseJSONOutput(t *testing.T) {
	if useJSONOutput(false) {
		t.Fatal("useJSONOutput(false) = true, want false without --json")
	}
	if !useJSONOutput(true) {
		t.Fatal("useJSONOutput(true) = false, want true when --json is enabled")
	}
}
