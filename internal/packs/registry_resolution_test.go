package packs_test

import (
	"reflect"
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestResolveEnabledSet(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	mustRegisterPack(t, reg, "database.postgresql")
	mustRegisterPack(t, reg, "database.mysql")
	mustRegisterPack(t, reg, "core.git")

	tests := []struct {
		name     string
		enabled  []string
		disabled []string
		wantIDs  []string
		wantSet  map[string]bool
	}{
		{
			name:     "category expansion deduplicates and keeps order",
			enabled:  []string{"database", "core.git", "database", "unknown", "core.git"},
			disabled: []string{"database", "unknown"},
			wantIDs:  []string{"core.git"},
			wantSet:  map[string]bool{"core.git": true},
		},
		{
			name:     "disabled pack overrides enabled category membership",
			enabled:  []string{"database", "core.git"},
			disabled: []string{"database.mysql"},
			wantIDs:  []string{"database.postgresql", "core.git"},
			wantSet:  map[string]bool{"database.postgresql": true, "core.git": true},
		},
		{
			name:     "unknown ids are preserved when not disabled",
			enabled:  []string{"unknown", "core.git", "unknown"},
			disabled: []string{"database"},
			wantIDs:  []string{"unknown", "core.git"},
			wantSet:  map[string]bool{"unknown": true, "core.git": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIDs, gotSet := reg.ResolveEnabledSet(tt.enabled, tt.disabled)
			if !reflect.DeepEqual(gotIDs, tt.wantIDs) {
				t.Fatalf("ResolveEnabledSet() ids = %v, want %v", gotIDs, tt.wantIDs)
			}
			if !reflect.DeepEqual(gotSet, tt.wantSet) {
				t.Fatalf("ResolveEnabledSet() set = %v, want %v", gotSet, tt.wantSet)
			}
		})
	}
}

func TestExpandEnabled(t *testing.T) {
	reg := packs.NewEmptyRegistry()
	mustRegisterPack(t, reg, "database.postgresql")
	mustRegisterPack(t, reg, "database.mysql")
	mustRegisterPack(t, reg, "core.git")

	tests := []struct {
		name    string
		input   []string
		wantIDs []string
	}{
		{
			name:    "expands category in registry order",
			input:   []string{"database"},
			wantIDs: []string{"database.postgresql", "database.mysql"},
		},
		{
			name:    "deduplicates repeated category and explicit ids",
			input:   []string{"database", "core.git", "database", "core.git"},
			wantIDs: []string{"database.postgresql", "database.mysql", "core.git"},
		},
		{
			name:    "passes through unknown ids unchanged",
			input:   []string{"unknown", "database.postgresql"},
			wantIDs: []string{"unknown", "database.postgresql"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotIDs := reg.ExpandEnabled(tt.input); !reflect.DeepEqual(gotIDs, tt.wantIDs) {
				t.Fatalf("ExpandEnabled() = %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

func TestCategoryOf(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "single segment", id: "coregit", want: "coregit"},
		{name: "simple category", id: "database.postgresql", want: "database"},
		{name: "multiple dots", id: "platform.gitlab.ci", want: "platform"},
		{name: "leading dot", id: ".hidden", want: ""},
		{name: "trailing dot", id: "category.", want: "category"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packs.CategoryOf(tt.id); got != tt.want {
				t.Fatalf("CategoryOf(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func mustRegisterPack(t *testing.T, reg *packs.PackRegistry, id string) {
	t.Helper()

	if err := reg.RegisterPack(&packs.Pack{
		ID:          id,
		Name:        id,
		Description: id,
		Keywords:    []string{"test"},
		StructuralPatterns: []packs.StructuralPattern{
			{
				Name:     id + "-pattern",
				Reason:   id + " reason",
				Severity: packs.SeverityHigh,
				When: packs.PatternCondition{
					Command: []string{"tw"},
				},
			},
		},
	}, "test"); err != nil {
		t.Fatalf("RegisterPack(%q): %v", id, err)
	}
}
