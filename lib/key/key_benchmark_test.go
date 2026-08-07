package key_test

import (
	"testing"

	"github.com/linkdata/jaws/lib/key"
)

var appendBenchSink []byte

// BenchmarkAppend guards the base-32 encode hot path; it must stay
// allocation-light when appending into an existing buffer.
func BenchmarkAppend(b *testing.B) {
	const benchmarkKey key.Key = 0x1234abcd
	b.ReportAllocs()
	// Seed the backing array before B.Loop resets the timer so every measured
	// append uses an existing buffer, including fixed-iteration runs.
	appendBenchSink = key.Append(appendBenchSink[:0], benchmarkKey)
	for b.Loop() {
		appendBenchSink = key.Append(appendBenchSink[:0], benchmarkKey)
	}
}
