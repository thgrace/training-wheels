package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecuteRunsVersionCommand(t *testing.T) {
	t.Cleanup(resetRootCommandState)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"version", "--json"})

	if err := Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.TrimSpace(stderr.String()) != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var out struct {
		Version string `json:"version"`
		GOOS    string `json:"goos"`
		GOARCH  string `json:"goarch"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not valid version json: %v\nstdout=%q", err, stdout.String())
	}
	if out.Version == "" || out.GOOS == "" || out.GOARCH == "" {
		t.Fatalf("version output missing fields: %+v", out)
	}
}

func TestRootCommandWiring(t *testing.T) {
	if rootCmd.Use != "tw" {
		t.Fatalf("rootCmd.Use = %q, want tw", rootCmd.Use)
	}
	if rootCmd.PersistentFlags().Lookup("verbose") == nil {
		t.Fatal("missing persistent --verbose flag")
	}
	if rootCmd.PersistentFlags().Lookup("quiet") == nil {
		t.Fatal("missing persistent --quiet flag")
	}

	got := make(map[string]bool, len(rootCmd.Commands()))
	for _, cmd := range rootCmd.Commands() {
		got[cmd.Name()] = true
	}

	for _, want := range []string{"test", "packs", "rule", "version", "completions"} {
		if !got[want] {
			t.Fatalf("root command missing subcommand %q", want)
		}
	}
	if completionsCmd.Hidden {
		t.Fatal("expected completions command to be discoverable")
	}
}

func resetRootCommandState() {
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	rootCmd.SetArgs(nil)
	verbose = false
	quiet = false
	versionJSON = false
}
