package named

import (
	"html/template"
	"sync"
	"testing"
)

// TestBoolArray_ConcurrentLockOrdering exercises array reads and writes alongside
// [Bool.JawsSet], which acquires the array lock before the Bool lock.
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
	selected := nba.data[0] // save the pointer before WriteLocked clears old slices

	const goroutines = 16
	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range iterations {
				name := names[(g+i)%len(names)]
				// Exercise Set, WriteLocked, both JawsSet paths, Add,
				// JawsContains, and reads. The locked callbacks touch
				// only the provided slice and Bool methods, honoring the
				// non-reentrancy contract.
				switch i % 12 {
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
				case 8:
					_ = selected.JawsSet(elem, (g+i)%2 == 0)
				case 9:
					nba.Add(name, template.HTML(name))
				case 10:
					_ = nba.JawsContains(elem)
				case 11:
					_ = nba.JawsSetValues(elem, []string{name})
					_ = nba.JawsGetValues(elem)
				}
			}
		}(g)
	}
	wg.Wait()
}
