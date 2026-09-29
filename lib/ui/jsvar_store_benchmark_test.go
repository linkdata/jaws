package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
)

var jsVarBenchmarkPatch string

// BenchmarkJsVarStoreManyBindings measures one changed leaf projected to every
// binding of a shared store.
func BenchmarkJsVarStoreManyBindings(b *testing.B) {
	for _, count := range []int{1, 100} {
		b.Run("bindings="+strconv.Itoa(count), func(b *testing.B) {
			jw, err := jaws.New()
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(jw.Close)
			players := make(map[string]map[string]int, 512)
			for i := range 512 {
				players["player"+strconv.Itoa(i)] = map[string]int{"x": i}
			}
			players["alice"] = map[string]int{"x": -1}
			state := map[string]map[string]map[string]int{"players": players}
			var mu sync.RWMutex
			store, err := NewJsVarStore(jw, "client", &mu, &state)
			if err != nil {
				b.Fatal(err)
			}
			bindings := make([]*JsVarBinding[map[string]map[string]map[string]int], count)
			for i := range bindings {
				bindings[i] = store.Bind()
			}
			root, err := json.Marshal(state)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				if _, err := store.SetPath("players.alice.x", i); err != nil {
					b.Fatal(err)
				}
				for _, binding := range bindings {
					patches, err := binding.pendingPatches()
					if err != nil || len(patches) != 1 {
						b.Fatalf("pendingPatches = %q, %v", patches, err)
					}
					jsVarBenchmarkPatch = patches[0]
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(len(root)), "root_B")
		})
	}
}

// BenchmarkJsVarStorePendingParallel measures concurrent bindings reading the
// same changed version, including contention on the shared snapshot.
func BenchmarkJsVarStorePendingParallel(b *testing.B) {
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(jw.Close)
	players := make(map[string]map[string]int, 512)
	for i := range 512 {
		players["player"+strconv.Itoa(i)] = map[string]int{"x": i}
	}
	players["alice"] = map[string]int{"x": -1}
	state := map[string]map[string]map[string]int{"players": players}
	var mu sync.RWMutex
	store, err := NewJsVarStore(jw, "client", &mu, &state)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := store.SetPath("players.alice.x", 0); err != nil {
		b.Fatal(err)
	}
	var failed atomic.Bool
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		binding := store.Bind()
		for pb.Next() {
			binding.lastVersion.Store(0)
			patches, err := binding.pendingPatches()
			if err != nil || len(patches) != 1 {
				failed.Store(true)
			}
		}
	})
	if failed.Load() {
		b.Fatal("pendingPatches failed")
	}
}

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

// BenchmarkJsVarStoreSnapshotPatches measures multi-path projection and writer
// lock wait while a Request encodes its current store value.
func BenchmarkJsVarStoreSnapshotPatches(b *testing.B) {
	for _, pathCount := range []int{1, 64} {
		for _, parallel := range []bool{false, true} {
			name := "paths=" + strconv.Itoa(pathCount)
			if parallel {
				name += "/parallel"
			}
			b.Run(name, func(b *testing.B) {
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
				var mu sync.Mutex
				store, err := NewJsVarStore(jw, "players", &mu, &state)
				if err != nil {
					b.Fatal(err)
				}
				binding := store.Bind()
				paths := make([]string, pathCount)
				for i := range paths {
					paths[i] = "players.absent" + strconv.Itoa(i)
				}
				b.ReportAllocs()
				b.ResetTimer()
				if parallel {
					var iteration, waits, writes atomic.Uint64
					var failed atomic.Bool
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							if iteration.Add(1)%8 == 0 {
								start := time.Now()
								mu.Lock()
								waits.Add(uint64(time.Since(start).Nanoseconds()))
								writes.Add(1)
								players["alice"]["x"]++
								mu.Unlock()
							} else if _, err := binding.snapshotPatches(paths); err != nil {
								failed.Store(true)
							}
						}
					})
					if failed.Load() {
						b.Fatal("snapshotPatches failed")
					}
					if writes.Load() > 0 {
						b.ReportMetric(float64(waits.Load())/float64(writes.Load()), "writer_wait_ns")
					}
				} else {
					for range b.N {
						if _, err := binding.snapshotPatches(paths); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

// BenchmarkJsVarStoreProposal measures a changed-leaf proposal through a rendered binding.
func BenchmarkJsVarStoreProposal(b *testing.B) {
	for _, parallel := range []bool{false, true} {
		name := "serial"
		if parallel {
			name = "parallel"
		}
		b.Run(name, func(b *testing.B) {
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
			var mu sync.Mutex
			store, err := NewJsVarStore(jw, "players", &mu, &state)
			if err != nil {
				b.Fatal(err)
			}
			store.ClientCheck = func(*jaws.Element, *map[string]map[string]map[string]int, string) error { return nil }
			binding := store.Bind()
			rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			elem := rq.NewElement(binding)
			if err := elem.JawsRender(io.Discard, nil); err != nil {
				b.Fatal(err)
			}
			data, err := json.Marshal(state)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(float64(len(data)), "state_B")
			b.ReportAllocs()
			b.ResetTimer()
			if parallel {
				var iteration atomic.Uint64
				var failed atomic.Bool
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						value := float64(iteration.Add(1))
						if _, err := binding.applyProposal(elem, "players.alice.x", value); err != nil {
							failed.Store(true)
						}
					}
				})
				if failed.Load() {
					b.Fatal("applyProposal failed")
				}
			} else {
				for i := range b.N {
					if _, err := binding.applyProposal(elem, "players.alice.x", float64(i)); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
