package jawstree

import (
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

var treeRenderSink string

// BenchmarkTreeJawsRender measures rendering a Tree into a fresh buffer and its
// real Request queue.
func BenchmarkTreeJawsRender(b *testing.B) {
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()
	<-tr.ReadyCh

	root := &Node{Children: []*Node{{Name: "Documents"}}}
	var mu deadlock.RWMutex
	tree := New("benchmark_tree", ui.NewJsVar(&mu, root), InitiallyExpanded|SearchEnabled)

	b.ReportAllocs()
	b.ResetTimer()
	elems := make([]*jaws.Element, 0, 64)
	for renderedCount := 0; renderedCount < b.N; {
		b.StopTimer()
		elems = elems[:0]
		for range min(64, b.N-renderedCount) {
			elems = append(elems, tr.NewElement(tree))
		}
		b.StartTimer()

		for _, elem := range elems {
			var rendered strings.Builder
			if err = tree.JawsRender(elem, &rendered, nil); err != nil {
				b.Fatal(err)
			}
			treeRenderSink = rendered.String()
		}

		b.StopTimer()
		// Queue a same-element marker after any initializer and wake the live
		// Request loop. Draining through that marker leaves the real queue empty for
		// the next measured render on both the script and Call implementations.
		elems[len(elems)-1].SetValue("benchmark-drain")
		tr.InCh <- wire.WsMsg{}
		for {
			msg := <-tr.OutCh
			if msg.What == what.Value {
				break
			}
		}
		for _, elem := range elems {
			tr.DeleteElement(elem)
		}
		renderedCount += len(elems)
		b.StartTimer()
	}
	b.StopTimer()
}
