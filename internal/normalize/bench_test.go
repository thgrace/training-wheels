package normalize

import "testing"

func BenchmarkPreNormalize_Simple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PreNormalize("git status")
	}
}

func BenchmarkPreNormalize_Complex(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PreNormalize(`git -C "/path/to/repo" commit -m "fix: update things" 2>/dev/null`)
	}
}

func BenchmarkNormalizeCommand_Sudo(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NormalizeCommand("sudo -u deploy git pull origin main")
	}
}

func BenchmarkNormalizeCommand_Env(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NormalizeCommand("env NODE_ENV=production node server.js")
	}
}

func BenchmarkNormalizeCommand_Backslash(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NormalizeCommand(`\rm -rf /tmp/junk`)
	}
}
