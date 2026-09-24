package jaws

import (
	"strconv"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// BenchmarkSetBatchDistinctPaths measures internal broadcasts per distinct path.
func BenchmarkSetBatchDistinctPaths(b *testing.B) {
	const paths = 20
	msgs := make([]wire.Message, paths)
	for i := range msgs {
		path := "f" + strconv.Itoa(i)
		msgs[i] = wire.Message{Dest: tag.Tag("state"), What: what.Set, Data: path + "=" + strconv.Itoa(i)}
	}
	b.ReportAllocs()
	var broadcasts int
	for b.Loop() {
		var batch setBatch
		for _, msg := range msgs {
			batch.add(msg)
		}
		batch.flush(func(wire.Message) { broadcasts++ })
	}
	b.ReportMetric(float64(broadcasts)/float64(b.N*paths), "broadcasts/path")
}
