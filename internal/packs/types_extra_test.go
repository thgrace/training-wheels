package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestSeverityStringAndParse(t *testing.T) {
	tests := []struct {
		name       string
		severity   packs.Severity
		wantString string
		parseInput string
		wantParse  packs.Severity
		wantErr    bool
	}{
		{
			name:       "critical",
			severity:   packs.SeverityCritical,
			wantString: "critical",
			parseInput: "critical",
			wantParse:  packs.SeverityCritical,
		},
		{
			name:       "high",
			severity:   packs.SeverityHigh,
			wantString: "high",
			parseInput: " high ",
			wantParse:  packs.SeverityHigh,
		},
		{
			name:       "medium",
			severity:   packs.SeverityMedium,
			wantString: "medium",
			parseInput: "MEDIUM",
			wantParse:  packs.SeverityMedium,
		},
		{
			name:       "low",
			severity:   packs.SeverityLow,
			wantString: "low",
			parseInput: "low",
			wantParse:  packs.SeverityLow,
		},
		{
			name:       "unknown severity value",
			severity:   packs.Severity(99),
			wantString: "unknown",
			parseInput: "unsupported",
			wantParse:  packs.SeverityLow,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.wantString {
				t.Fatalf("Severity.String() = %q, want %q", got, tt.wantString)
			}

			got, err := packs.ParseSeverity(tt.parseInput)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSeverity(%q) error = nil, want error", tt.parseInput)
				}
			} else if err != nil {
				t.Fatalf("ParseSeverity(%q) unexpected error: %v", tt.parseInput, err)
			}
			if got != tt.wantParse {
				t.Fatalf("ParseSeverity(%q) = %v, want %v", tt.parseInput, got, tt.wantParse)
			}
		})
	}
}

func TestPlatformStringAndParse(t *testing.T) {
	tests := []struct {
		name       string
		platform   packs.Platform
		wantString string
		parseInput string
		wantParse  packs.Platform
		wantErr    bool
	}{
		{
			name:       "all",
			platform:   packs.PlatformAll,
			wantString: "all",
			parseInput: "",
			wantParse:  packs.PlatformAll,
		},
		{
			name:       "linux",
			platform:   packs.PlatformLinux,
			wantString: "linux",
			parseInput: " linux ",
			wantParse:  packs.PlatformLinux,
		},
		{
			name:       "macos",
			platform:   packs.PlatformMacOS,
			wantString: "macos",
			parseInput: "MACOS",
			wantParse:  packs.PlatformMacOS,
		},
		{
			name:       "windows",
			platform:   packs.PlatformWindows,
			wantString: "windows",
			parseInput: "windows",
			wantParse:  packs.PlatformWindows,
		},
		{
			name:       "bsd",
			platform:   packs.PlatformBSD,
			wantString: "bsd",
			parseInput: "bsd",
			wantParse:  packs.PlatformBSD,
		},
		{
			name:       "unknown platform value",
			platform:   packs.Platform(99),
			wantString: "all",
			parseInput: "solaris",
			wantParse:  packs.PlatformAll,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.platform.String(); got != tt.wantString {
				t.Fatalf("Platform.String() = %q, want %q", got, tt.wantString)
			}

			got, err := packs.ParsePlatform(tt.parseInput)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePlatform(%q) error = nil, want error", tt.parseInput)
				}
			} else if err != nil {
				t.Fatalf("ParsePlatform(%q) unexpected error: %v", tt.parseInput, err)
			}
			if got != tt.wantParse {
				t.Fatalf("ParsePlatform(%q) = %v, want %v", tt.parseInput, got, tt.wantParse)
			}
		})
	}
}
