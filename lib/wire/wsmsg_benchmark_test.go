package wire

import (
	"testing"

	"github.com/linkdata/jaws/lib/what"
)

var (
	parseBenchSink  WsMsg
	appendBenchSink []byte
)

// BenchmarkAppend guards the outbound frame-encoding hot path; it must stay
// allocation-light.
func BenchmarkAppend(b *testing.B) {
	m := WsMsg{
		Data: "name",
		Jid:  1,
		What: what.Alert,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		appendBenchSink = m.Append(nil)
	}
}

// BenchmarkParse guards the inbound parse hot path (run on every WebSocket frame)
// across the common command shapes, including the lone-surrogate case that decodes
// via the JSON fallback rather than being dropped. The common quoted and unquoted
// paths must stay allocation-light; only the rare surrogate case pays the
// JSON-decode cost.
func BenchmarkParse(b *testing.B) {
	frames := []struct {
		name  string
		frame []byte
	}{
		{"input_plain", []byte("Input\tJid.1\t\"hello world\"\n")},
		{"input_escaped", []byte("Input\tJid.1\t\"a\\nb\\tc\"\n")},
		{"input_surrogate", []byte("Input\tJid.1\t\"\\ud800\"\n")},
		{"input_unquoted", []byte("Input\tJid.1\ttrue\n")},
		{"set_verbatim", []byte("Set\tJid.1\tpath={\"a\":1}\n")},
	}
	for _, f := range frames {
		b.Run(f.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parseBenchSink, _ = Parse(f.frame)
			}
		})
	}
}
