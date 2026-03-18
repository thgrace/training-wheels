package packs

import "testing"

func BenchmarkLazyRegex_FirstCall_RE2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lr := NewLazyRegex(`rm\s+-[a-zA-Z]*[rR][a-zA-Z]*f`)
		lr.IsMatch("rm -rf /tmp")
	}
}

func BenchmarkLazyRegex_FirstCall_Backtracking(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lr := NewLazyRegex(`git\s+(?:\S+\s+)*restore\s+(?!--staged\b)(?!-S\b)`)
		lr.IsMatch("git restore .")
	}
}

func BenchmarkLazyRegex_CachedCall_RE2(b *testing.B) {
	lr := NewLazyRegex(`rm\s+-[a-zA-Z]*[rR][a-zA-Z]*f`)
	lr.IsMatch("") // warm up
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lr.IsMatch("rm -rf /tmp")
	}
}

func BenchmarkLazyRegex_CachedCall_Backtracking(b *testing.B) {
	lr := NewLazyRegex(`git\s+(?:\S+\s+)*restore\s+(?!--staged\b)(?!-S\b)`)
	lr.IsMatch("") // warm up
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lr.IsMatch("git restore .")
	}
}
