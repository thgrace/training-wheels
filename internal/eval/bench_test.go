package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/packs"
)

func BenchmarkNewEvaluator(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewEvaluator(cfg, reg)
	}
}

func BenchmarkNewEvaluator_DefaultConfig(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewEvaluator(cfg, reg)
	}
}

func BenchmarkEvaluate_Allow_NoKeywords(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, "ls -la")
	}
}

func BenchmarkEvaluate_Allow_NoKeywords_DefaultConfig(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, "ls -la")
	}
}

func BenchmarkEvaluate_Allow_WithKeywords(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, "git status")
	}
}

func BenchmarkEvaluate_Allow_WithKeywords_DefaultConfig(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, "git status")
	}
}

func BenchmarkEvaluate_Deny(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, "git reset --hard HEAD~1")
	}
}

func BenchmarkEvaluate_Deny_DefaultConfig(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, "git reset --hard HEAD~1")
	}
}

func BenchmarkEvaluate_LongCommand(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	ctx := context.Background()
	// 500-byte command with git keyword
	cmd := "git " + strings.Repeat("-C /some/path/here ", 25) + "status"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Evaluate(ctx, cmd)
	}
}

func BenchmarkQuickReject_Allow(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.kwIndex.QuickReject("ls -la /tmp")
	}
}

func BenchmarkQuickReject_Candidate(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.kwIndex.QuickReject("git status")
	}
}

func BenchmarkQuickReject_LongCommand(b *testing.B) {
	cfg := config.DefaultConfig()
	reg := packs.DefaultRegistry()
	cfg.Packs.Enabled = reg.AllIDs()
	e := NewEvaluator(cfg, reg)
	cmd := strings.Repeat("some-word ", 50) + "git status"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.kwIndex.QuickReject(cmd)
	}
}
