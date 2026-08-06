package key_test

import (
	"testing"

	"github.com/linkdata/jaws/lib/key"
)

var appendBenchSink []byte

// BenchmarkAppend guards the base-32 encode hot path; it must stay
// allocation-light when appending into an existing buffer.
func BenchmarkAppend(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		appendBenchSink = key.Append(appendBenchSink[:0], key.Key(0x1234abcd))
	}
}
