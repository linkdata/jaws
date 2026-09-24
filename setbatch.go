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

func sameSetMessageDest(a, b any) bool {
	if tags, ok := a.([]any); ok {
		other, ok := b.([]any)
		return ok && sameSetDest(tags, other)
	}
	if _, ok := b.([]any); ok {
		return false
	}
	return a == b
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

func (batch *setBatch) flush(send func(wire.Message)) {
	slices.SortFunc(batch.entries, func(a, b setBatchEntry) int {
		return cmp.Compare(a.order, b.order)
	})
	for i := 0; i < len(batch.entries); {
		msg := batch.entries[i].msg
		j := i + 1
		dataLen := len(msg.Data)
		for j < len(batch.entries) && sameSetMessageDest(msg.Dest, batch.entries[j].msg.Dest) {
			dataLen += 1 + len(batch.entries[j].msg.Data)
			j++
		}
		if j > i+1 {
			var data strings.Builder
			data.Grow(dataLen)
			for k := i; k < j; k++ {
				if k > i {
					data.WriteByte('\n')
				}
				data.WriteString(batch.entries[k].msg.Data)
			}
			// Newlines separate Sets only inside the server. handleBroadcast
			// splits them back into individual WebSocket messages.
			msg.Data = data.String()
		}
		send(msg)
		i = j
	}
	batch.index = nil
	batch.entries = nil
	batch.order = 0
}
