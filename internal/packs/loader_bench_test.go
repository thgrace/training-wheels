package packs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/packs/allpacks"
	packassets "github.com/thgrace/training-wheels/packs"
)

func BenchmarkRegisterAll_Builtins(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reg := packs.NewEmptyRegistry()
		allpacks.RegisterAll(reg)
		if reg.Count() == 0 {
			b.Fatal("expected built-in packs to load")
		}
	}
}

func BenchmarkLoadFromDir_Builtins(b *testing.B) {
	dir := materializeEmbeddedJSONDir(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg := packs.NewEmptyRegistry()
		if err := reg.LoadFromDir(dir); err != nil {
			b.Fatal(err)
		}
		if reg.Count() == 0 {
			b.Fatal("expected packs to load from directory")
		}
	}
}

func BenchmarkLoadFile_DatabaseCategory(b *testing.B) {
	path := materializeEmbeddedJSONFile(b, "json/database.json")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg := packs.NewEmptyRegistry()
		if err := reg.LoadFile(path); err != nil {
			b.Fatal(err)
		}
		if reg.Count() == 0 {
			b.Fatal("expected database category packs to load")
		}
	}
}

func materializeEmbeddedJSONDir(tb testing.TB) string {
	tb.Helper()

	matches, err := fs.Glob(packassets.Files, packassets.BuiltinJSONPattern)
	if err != nil {
		tb.Fatalf("fs.Glob: %v", err)
	}
	sort.Strings(matches)

	dir := tb.TempDir()
	for _, name := range matches {
		writeEmbeddedJSONFile(tb, dir, name)
	}
	return dir
}

func materializeEmbeddedJSONFile(tb testing.TB, embeddedPath string) string {
	tb.Helper()

	dir := tb.TempDir()
	return writeEmbeddedJSONFile(tb, dir, embeddedPath)
}

func writeEmbeddedJSONFile(tb testing.TB, dir, embeddedPath string) string {
	tb.Helper()

	data, err := fs.ReadFile(packassets.Files, embeddedPath)
	if err != nil {
		tb.Fatalf("fs.ReadFile(%s): %v", embeddedPath, err)
	}

	path := filepath.Join(dir, filepath.Base(embeddedPath))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		tb.Fatalf("WriteFile(%s): %v", path, err)
	}
	return path
}
