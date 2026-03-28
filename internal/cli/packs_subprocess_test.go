package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTWPacksCommandFlags(t *testing.T) {
	twBinary := buildTWBinary(t)

	t.Run("json always includes full detail", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "packs", "--json")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}

		var out []struct {
			ID           string   `json:"id"`
			Patterns     []string `json:"patterns"`
			SafePatterns []string `json:"safe_patterns"`
		}
		if err := json.Unmarshal([]byte(stdout), &out); err != nil {
			t.Fatalf("stdout is not valid packs json: %v\nstdout=%q", err, stdout)
		}

		for _, pack := range out {
			if pack.ID != "core.git" {
				continue
			}
			if len(pack.Patterns) == 0 {
				t.Fatalf("core.git patterns are missing from json output: %+v", pack)
			}
			if len(pack.SafePatterns) == 0 {
				t.Fatalf("core.git safe_patterns are missing from json output: %+v", pack)
			}
			return
		}

		t.Fatalf("core.git pack not found in output")
	})

	t.Run("pretty output stays summary only", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "packs")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if !strings.Contains(stdout, "PACK ID") {
			t.Fatalf("stdout missing packs header: %q", stdout)
		}
		if strings.Contains(stdout, "description:") {
			t.Fatalf("stdout unexpectedly contains description details: %q", stdout)
		}
		if strings.Contains(stdout, "destructive:") {
			t.Fatalf("stdout unexpectedly contains destructive pattern details: %q", stdout)
		}
	})

	t.Run("details flag is rejected", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "packs", "--details")
		if exitCode == 0 {
			t.Fatalf("exit code = 0, want non-zero\nstdout=%q\nstderr=%q", stdout, stderr)
		}
		if !strings.Contains(stderr, "unknown flag: --details") {
			t.Fatalf("stderr missing unknown flag error: %q", stderr)
		}
	})

	t.Run("global verbose shorthand works on packs", func(t *testing.T) {
		stdout, stderr, exitCode := runTWCommand(t, twBinary, "packs", "-v")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
		}
		if !strings.Contains(stdout, "PACK ID") {
			t.Fatalf("stdout missing packs header: %q", stdout)
		}
	})
}
