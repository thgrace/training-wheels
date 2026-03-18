package shellcontext

import "testing"

func BenchmarkClassify_Simple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Classify("git status", nil)
	}
}

func BenchmarkClassify_InlineShell(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Classify(`bash -c "rm -rf /tmp/test"`, nil)
	}
}

func BenchmarkSanitize_DataMasking(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Sanitize(`echo "rm -rf /" | git commit -m "drop table users"`, nil)
	}
}

func BenchmarkSanitize_NoMasking(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Sanitize("git status", nil)
	}
}
