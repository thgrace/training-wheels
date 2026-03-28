package cli

import (
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/thgrace/training-wheels/internal/app"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/packs"
	packassets "github.com/thgrace/training-wheels/packs"
)

func BenchmarkCLIStartup_Default(b *testing.B) {
	home := b.TempDir()
	projectDir := b.TempDir()
	setBenchmarkHome(b, home)
	chdirBenchmark(b, projectDir)
	logger.Configure(io.Discard, slog.LevelWarn, logger.FormatText)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg, err := config.Load()
		if err != nil {
			b.Fatal(err)
		}
		reg := packs.NewEmptyRegistry()
		app.InitializeRegistry(reg, cfg)
		if reg.Count() == 0 {
			b.Fatal("expected built-in packs to load")
		}
	}
}

func BenchmarkCLIStartup_WithExternalPacks(b *testing.B) {
	home := b.TempDir()
	projectDir := b.TempDir()
	customDir := filepath.Join(projectDir, "custom-packs")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		b.Fatal(err)
	}

	setBenchmarkHome(b, home)
	chdirBenchmark(b, projectDir)
	logger.Configure(io.Discard, slog.LevelWarn, logger.FormatText)

	userDir, err := config.UserPackDir()
	if err != nil {
		b.Fatal(err)
	}
	projectPackDir := filepath.Join(projectDir, config.ProjectPackDir())
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(projectPackDir, 0o755); err != nil {
		b.Fatal(err)
	}

	writePackFile(b, filepath.Join(userDir, "user-extra.json"), `{
  "category": "userextra",
  "packs": [
    {
      "id": "userextra.one",
      "name": "User Extra",
      "description": "benchmark user pack",
      "keywords": ["userextra"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-userextra", "regex": "^userextra$", "reason": "userextra", "severity": "high"}
      ]
    }
  ]
}`)
	writePackFile(b, filepath.Join(projectPackDir, "project-extra.json"), `{
  "category": "projectextra",
  "packs": [
    {
      "id": "projectextra.one",
      "name": "Project Extra",
      "description": "benchmark project pack",
      "keywords": ["projectextra"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-projectextra", "regex": "^projectextra$", "reason": "projectextra", "severity": "medium"}
      ]
    }
  ]
}`)
	writePackFile(b, filepath.Join(customDir, "custom-extra.json"), `{
  "category": "customextra",
  "packs": [
    {
      "id": "customextra.one",
      "name": "Custom Extra",
      "description": "benchmark custom pack",
      "keywords": ["customextra"],
      "safe_patterns": [],
      "destructive_patterns": [
        {"name": "block-customextra", "regex": "^customextra$", "reason": "customextra", "severity": "low"}
      ]
    }
  ]
}`)
	// Use env var for custom pack paths (project-level .tw.json no longer supported).
	b.Setenv("TW_PACKS_PATHS", customDir)
	// Create .git so FindProjectRoot resolves project pack dir.
	if err := os.Mkdir(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg, err := config.Load()
		if err != nil {
			b.Fatal(err)
		}
		reg := packs.NewEmptyRegistry()
		app.InitializeRegistry(reg, cfg)
		if reg.Get("userextra.one") == nil || reg.Get("projectextra.one") == nil || reg.Get("customextra.one") == nil {
			b.Fatal("expected external packs to load")
		}
	}
}

func BenchmarkCLIStartup_ExternalDirectoryScanOnly(b *testing.B) {
	home := b.TempDir()
	projectDir := b.TempDir()
	setBenchmarkHome(b, home)
	chdirBenchmark(b, projectDir)
	logger.Configure(io.Discard, slog.LevelWarn, logger.FormatText)

	userDir, err := config.UserPackDir()
	if err != nil {
		b.Fatal(err)
	}
	projectPackDir := filepath.Join(projectDir, config.ProjectPackDir())
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(projectPackDir, 0o755); err != nil {
		b.Fatal(err)
	}
	materializeEmbeddedJSONDir(b, userDir)

	cfg, err := config.Load()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg := packs.NewEmptyRegistry()
		app.LoadPackSourcesFromConfig(reg, cfg)
	}
}

func setBenchmarkHome(tb testing.TB, home string) {
	tb.Helper()
	tb.Setenv("HOME", home)
	tb.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func chdirBenchmark(tb testing.TB, dir string) {
	tb.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}

func materializeEmbeddedJSONDir(tb testing.TB, dir string) {
	tb.Helper()

	matches, err := fs.Glob(packassets.Files, packassets.BuiltinJSONPattern)
	if err != nil {
		tb.Fatalf("fs.Glob: %v", err)
	}
	sort.Strings(matches)

	for _, name := range matches {
		data, err := fs.ReadFile(packassets.Files, name)
		if err != nil {
			tb.Fatalf("fs.ReadFile(%s): %v", name, err)
		}
		path := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			tb.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
}

func writePackFile(tb testing.TB, path, contents string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		tb.Fatal(err)
	}
}
