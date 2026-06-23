package named

import (
	"html/template"
	"sync"
	"testing"
)

// TestBoolArray_ConcurrentLockOrdering hammers a shared [BoolArray] from many
// goroutines to exercise its documented lock discipline under -race: the array
// mutex is taken before any per-[Bool] work, and the array mutex is released
// before [BoolArray.JawsSet] takes the jaws element lock to dirty values. That
// ordering is otherwise stated only in prose, so -race passes only incidentally
// without a test that actually contends the lock.
//
// Real goroutines are used deliberately, not testing/synctest: synctest serializes
// the goroutines in its bubble, which would hide data races and prevent the
// deadlock detector (active under -race) from observing concurrent lock acquisition
// order.
func TestBoolArray_ConcurrentLockOrdering(t *testing.T) {
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(noopUI{})

	nba := NewBoolArray(false)
	names := []string{"a", "b", "c", "d"}
	for _, n := range names {
		nba.Add(n, template.HTML(n))
	}

	const goroutines = 16
	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range iterations {
				name := names[(g+i)%len(names)]
				// Cover every public method that takes nba.mu: the write path
				// (Set, WriteLocked, JawsSet) and the read path (Get, Count,
				// IsChecked, String, ReadLocked, JawsGet). The Read/WriteLocked
				// callbacks touch only the provided slice and the Bool's own
				// methods, honoring the non-reentrancy contract.
				switch i % 8 {
				case 0:
					nba.Set(name, true)
				case 1:
					_ = nba.Get()
				case 2:
					_ = nba.Count(name)
				case 3:
					_ = nba.IsChecked(name)
				case 4:
					_ = nba.String()
				case 5:
					nba.ReadLocked(func(nbl []*Bool) {
						for _, nb := range nbl {
							_ = nb.Name()
						}
					})
				case 6:
					nba.WriteLocked(func(nbl []*Bool) []*Bool { return nbl })
				case 7:
					_ = nba.JawsSet(elem, name)
					_ = nba.JawsGet(elem)
				}
			}
		}(g)
	}
	wg.Wait()
}
