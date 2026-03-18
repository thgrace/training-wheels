package hook

import (
	"bytes"
	"testing"
)

func BenchmarkReadHookInput(b *testing.B) {
	payload := []byte(`{"tool_name":"bash","tool_input":{"command":"git status"}}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReadHookInput(bytes.NewReader(payload), 131072)
	}
}
