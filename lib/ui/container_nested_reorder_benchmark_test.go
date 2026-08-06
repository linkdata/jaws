package ui

import (
	"io"
	"strconv"
	"testing"

	"github.com/linkdata/jaws/lib/what"
)

// BenchmarkNestedContainersUpdate measures explicit updates at every level of a
// table/tbody/tr/td/select/option hierarchy whose providers rebuild equal widget values.
//
// Model permutation and wire draining run with the timer stopped. The timed section
// contains only reconciliation and queueing for the root and every nested container.
func BenchmarkNestedContainersUpdate(b *testing.B) {
	for _, benchmark := range []struct {
		name    string
		reorder bool
	}{
		{name: "unchanged"},
		{name: "reorder_every_level", reorder: true},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			b.StopTimer()
			tr := newReuseRequest(b)
			tree := newNestedReorderTree(2, 2, 2, 2, 3)
			root := tr.NewElement(NewContainer("table", tree))
			if err := root.JawsRender(io.Discard, nil); err != nil {
				b.Fatal(err)
			}
			elements := newNestedReorderElements(b, root, tree)
			elements.requireTree(b, tree)
			initial := nestedReorderDrainWire(b, tr, "nested benchmark initial render")
			requireNestedReorderWire(b, initial, elements.valueWireExpectations(tree))

			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				if benchmark.reorder {
					tree.reverse()
				}
				probe := "nested benchmark iteration " + strconv.Itoa(i)

				b.StartTimer()
				elements.updateAll()
				b.StopTimer()

				gotWire := nestedReorderDrainWire(b, tr, probe)
				var orders, values int
				for _, message := range gotWire {
					switch message.What {
					case what.Order:
						orders++
					case what.Value:
						values++
					default:
						b.Fatalf("unexpected wire mutation for %v: %v %q", message.Jid, message.What, message.Data)
					}
				}
				wantOrders := 0
				if benchmark.reorder {
					wantOrders = len(elements.updates)
				}
				if orders != wantOrders || values != len(elements.selects) {
					b.Fatalf("wire operations = %d Order and %d Value, want %d Order and %d Value",
						orders, values, wantOrders, len(elements.selects))
				}
			}
			elements.requireTree(b, tree)
		})
	}
}
