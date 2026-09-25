package jaws

import (
	"cmp"
	"slices"
	"strings"

	"github.com/linkdata/jaws/lib/wire"
)

type setBatchKey struct {
	dest any
	path string
}

type setBatchEntry struct {
	msg   wire.Message
	path  string
	order uint64
}

// setGroup carries one flush's Sets in write order; forRequest selects one Request's share.
type setGroup []wire.Message

func (group setGroup) forRequest(rq *Request) (selected setGroup) {
	selected = group
	for i := range group {
		if group[i].Dest == nil || rq.wantMessage(&group[i]) {
			continue
		}
		selected = append(setGroup(nil), group[:i]...)
		for j := i + 1; j < len(group); j++ {
			if group[j].Dest == nil || rq.wantMessage(&group[j]) {
				selected = append(selected, group[j])
			}
		}
		break
	}
	return
}

// setBatch keeps the last Set for each destination and path until a flush.
// It is owned by the Serve loop, which also owns broadcast order.
type setBatch struct {
	entries []setBatchEntry
	index   map[setBatchKey]int
	order   uint64
}

func sameSetDest(a, b []any) bool {
	// TagExpand returns unique comparable keys, and a TagGetter may reorder them.
	if len(a) != len(b) {
		return false
	}
	for _, tag := range a {
		if !slices.Contains(b, tag) {
			return false
		}
	}
	return true
}

func (batch *setBatch) add(msg wire.Message) (batched bool) {
	path, _, batched := strings.Cut(msg.Data, "=")
	if !batched {
		return
	}
	batch.order++
	if dest, ok := msg.Dest.([]any); ok {
		for i := range batch.entries {
			entry := &batch.entries[i]
			if other, ok := entry.msg.Dest.([]any); ok && entry.path == path && sameSetDest(dest, other) {
				entry.msg = msg
				entry.order = batch.order
				return
			}
		}
	} else {
		if batch.index == nil {
			batch.index = make(map[setBatchKey]int)
		}
		key := setBatchKey{dest: msg.Dest, path: path}
		if i, ok := batch.index[key]; ok {
			batch.entries[i].msg = msg
			batch.entries[i].order = batch.order
			return
		}
		batch.index[key] = len(batch.entries)
	}
	batch.entries = append(batch.entries, setBatchEntry{msg: msg, path: path, order: batch.order})
	return
}

func (batch *setBatch) take() (msgs setGroup) {
	slices.SortFunc(batch.entries, func(a, b setBatchEntry) int {
		return cmp.Compare(a.order, b.order)
	})
	if len(batch.entries) > 0 {
		msgs = make(setGroup, len(batch.entries))
		for i := range batch.entries {
			msgs[i] = batch.entries[i].msg
		}
	}
	batch.index = nil
	batch.entries = nil
	batch.order = 0
	return
}
