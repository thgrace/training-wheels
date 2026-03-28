package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestPrintPacksJSON(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	if err := reg.RegisterPack(&packs.Pack{
		ID:          "containers.docker",
		Name:        "Docker",
		Description: "Docker safety rules",
		Keywords:    []string{"docker", "compose"},
		StructuralPatterns: []packs.StructuralPattern{
			{Name: "docker-rm-force"},
		},
	}, "test"); err != nil {
		t.Fatalf("RegisterPack: %v", err)
	}

	var buf bytes.Buffer
	printPacksJSON(&buf, reg, []string{"containers.docker"}, map[string]bool{"containers.docker": true})

	var out []packJSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Name != "Docker" {
		t.Fatalf("Name = %q, want Docker", out[0].Name)
	}
	if len(out[0].Patterns) != 1 || out[0].Patterns[0] != "docker-rm-force" {
		t.Fatalf("Patterns = %v, want [docker-rm-force]", out[0].Patterns)
	}
}

func TestPrintPacksPrettyShowsSummaryOnly(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	if err := reg.RegisterPack(&packs.Pack{
		ID:          "containers.docker",
		Name:        "Docker",
		Description: "Docker safety rules",
		Keywords:    []string{"docker", "compose"},
		StructuralPatterns: []packs.StructuralPattern{
			{Name: "docker-rm-force"},
		},
	}, "test"); err != nil {
		t.Fatalf("RegisterPack: %v", err)
	}

	var buf bytes.Buffer
	printPacksPretty(&buf, reg, []string{"containers.docker"}, map[string]bool{"containers.docker": true})
	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("containers.docker")) {
		t.Fatalf("pretty output missing pack row:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("description:")) {
		t.Fatalf("pretty output unexpectedly contains description details:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("applies to:")) {
		t.Fatalf("pretty output unexpectedly contains applicability details:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("destructive:")) {
		t.Fatalf("pretty output unexpectedly contains destructive pattern details:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("safe:")) {
		t.Fatalf("pretty output unexpectedly contains safe pattern details:\n%s", got)
	}
}
