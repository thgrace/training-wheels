package packs_test

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
	packassets "github.com/thgrace/training-wheels/packs"
)

//go:embed testdata/embed/*.json
var embeddedPackFiles embed.FS

func TestLoadFromEmbed(t *testing.T) {
	reg := packs.NewEmptyRegistry()

	if err := reg.LoadFromEmbed(embeddedPackFiles, "testdata/embed/*.json"); err != nil {
		t.Fatalf("LoadFromEmbed: %v", err)
	}

	if got := reg.AllIDs(); len(got) != 2 {
		t.Fatalf("expected 2 embedded packs, got %d: %v", len(got), got)
	}
	if reg.Get("alpha.one") == nil {
		t.Fatal("alpha.one not loaded from embed")
	}
	if reg.Get("beta.one") == nil {
		t.Fatal("beta.one not loaded from embed")
	}
}

func TestBuiltinPackAssetsLoadCleanly(t *testing.T) {
	reg := packs.NewEmptyRegistry()

	if err := reg.LoadFromEmbed(packassets.Files, packassets.BuiltinJSONPattern); err != nil {
		t.Fatalf("LoadFromEmbed(builtins): %v", err)
	}
	if reg.Count() == 0 {
		t.Fatal("expected built-in packs to load")
	}
}

func TestLoadFileRejectsDuplicateIDsWithinFile(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	path := writePackFile(t, t.TempDir(), "dupe.json", `{
  "category": "demo",
  "packs": [
    {
      "id": "demo.same",
      "name": "First",
      "description": "first",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-first", "regex": "^demo first$", "reason": "first", "severity": "high"}
      ]
    },
    {
      "id": "demo.same",
      "name": "Second",
      "description": "second",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-second", "regex": "^demo second$", "reason": "second", "severity": "low"}
      ]
    }
  ]
}`)

	if err := reg.LoadFile(path); err == nil {
		t.Fatal("LoadFile succeeded for duplicate pack IDs in one file; want error")
	}
	if reg.Count() != 0 {
		t.Fatalf("expected file-level rejection to keep registry empty, got %d packs", reg.Count())
	}
}

func TestLoadFileRejectsRegistryDuplicatesButKeepsUniquePacks(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	if err := reg.RegisterPack(&packs.Pack{
		ID:          "demo.existing",
		Name:        "Existing",
		Description: "already loaded",
		Keywords:    []string{"demo"},
		DestructivePatterns: []packs.DestructivePattern{
			{
				Name:     "block-existing",
				Regex:    packs.NewLazyRegex("^existing$"),
				Reason:   "existing",
				Severity: packs.SeverityHigh,
			},
		},
	}, "test"); err != nil {
		t.Fatalf("RegisterPack: %v", err)
	}

	path := writePackFile(t, t.TempDir(), "mixed.json", `{
  "category": "demo",
  "packs": [
    {
      "id": "demo.existing",
      "name": "Replacement",
      "description": "should be ignored",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-replacement", "regex": "^replace$", "reason": "replace", "severity": "critical"}
      ]
    },
    {
      "id": "demo.unique",
      "name": "Unique",
      "description": "should load",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-unique", "regex": "^unique$", "reason": "unique", "severity": "medium"}
      ]
    }
  ]
}`)

	if err := reg.LoadFile(path); err == nil {
		t.Fatal("LoadFile succeeded without surfacing duplicate registry ID")
	}

	if reg.Count() != 2 {
		t.Fatalf("expected 2 packs after keeping existing + loading unique, got %d", reg.Count())
	}
	if got := reg.Get("demo.existing"); got == nil || got.Name != "Existing" {
		t.Fatalf("existing pack was replaced unexpectedly: %+v", got)
	}
	if reg.Get("demo.unique") == nil {
		t.Fatal("unique pack was not loaded")
	}
}

func TestLoadFromDirSkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	writePackFile(t, dir, "good.json", `{
  "category": "demo",
  "packs": [
    {
      "id": "demo.good",
      "name": "Good",
      "description": "loads",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-good", "regex": "^good$", "reason": "good", "severity": "high"}
      ]
    }
  ]
}`)
	writePackFile(t, dir, "bad.json", `{
  "category": "demo",
  "packs": [
    {
      "id": "wrong.prefix",
      "name": "Bad",
      "description": "rejected",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-bad", "regex": "^bad$", "reason": "bad", "severity": "high"}
      ]
    }
  ]
}`)

	reg := packs.NewEmptyRegistry()
	if err := reg.LoadFromDir(dir); err == nil {
		t.Fatal("LoadFromDir succeeded despite one invalid file; want aggregated error")
	}

	if reg.Get("demo.good") == nil {
		t.Fatal("valid pack was not loaded from directory")
	}
	if reg.Get("wrong.prefix") != nil {
		t.Fatal("invalid pack should not have been loaded")
	}
}

func TestLoadFileRejectsInvalidRegex(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	path := writePackFile(t, t.TempDir(), "lazy.json", `{
  "category": "demo",
  "packs": [
    {
      "id": "demo.lazy",
      "name": "Lazy",
      "description": "invalid regex is rejected at load time",
      "keywords": ["demo"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-lazy", "regex": "(", "reason": "invalid", "severity": "critical"}
      ]
    }
  ]
}`)

	if err := reg.LoadFile(path); err == nil {
		t.Fatal("LoadFile should reject invalid regex at load time")
	}

	if pack := reg.Get("demo.lazy"); pack != nil {
		t.Fatal("pack with invalid regex should not be in registry")
	}
}

func writePackFile(t *testing.T, dir, name, contents string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return path
}
