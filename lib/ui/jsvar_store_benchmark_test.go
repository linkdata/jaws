package ui

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

var jsVarBenchmarkPatch string

// BenchmarkJsVarStoreChangedLeaf measures a changed nested leaf and its
// canonical per-binding projection against a large JSON tree.
func BenchmarkJsVarStoreChangedLeaf(b *testing.B) {
	for _, bindings := range []int{1, 8} {
		b.Run("bindings="+strconv.Itoa(bindings), func(b *testing.B) {
			jw, err := jaws.New()
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(jw.Close)
			players := make(map[string]map[string]int, 1024)
			for i := range 1024 {
				players["player"+strconv.Itoa(i)] = map[string]int{"x": i}
			}
			players["alice"] = map[string]int{"x": -1}
			state := map[string]map[string]map[string]int{"players": players}
			var mu sync.RWMutex
			store, err := NewJsVarStore(jw, "players", &mu, &state)
			if err != nil {
				b.Fatal(err)
			}
			const path = "players.alice.x"
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				if _, err = store.SetPath(path, i); err != nil {
					b.Fatal(err)
				}
				for range bindings {
					store.locker.RLock()
					root, marshalErr := json.Marshal(store.value)
					if marshalErr == nil {
						jsVarBenchmarkPatch = store.projectPatch(path, root)
					}
					store.locker.RUnlock()
					if marshalErr != nil {
						b.Fatal(marshalErr)
					}
				}
			}
			b.StopTimer()
			if _, err = store.SetPath(path, 123456); err != nil {
				b.Fatal(err)
			}
			store.locker.RLock()
			root, err := json.Marshal(store.value)
			if err == nil {
				jsVarBenchmarkPatch = store.projectPatch(path, root)
			}
			store.locker.RUnlock()
			if err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(float64(len(root)), "root_B")
			b.ReportMetric(float64(len(jsVarBenchmarkPatch)), "patch_B")
		})
	}
}
