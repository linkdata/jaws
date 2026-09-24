package jaws

import (
	"strings"
	"sync"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// broadcastQueue keeps a bounded FIFO for one Request. Set frames for the same
// destination and path can supersede queued frames when the FIFO is full.
type broadcastQueue struct {
	mu     sync.Mutex
	msgs   []wire.Message
	head   int
	count  int
	ready  chan struct{}
	closed bool
}

func newBroadcastQueue(size int) *broadcastQueue {
	return &broadcastQueue{
		msgs:  make([]wire.Message, max(size, 1)),
		ready: make(chan struct{}, 1),
	}
}

// offer returns true only when a one-shot message overloads the queue.
func (q *broadcastQueue) offer(msg wire.Message) (overloaded bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	if q.count < len(q.msgs) {
		q.msgs[(q.head+q.count)%len(q.msgs)] = msg
		q.count++
		if q.count == 1 {
			select {
			case q.ready <- struct{}{}:
			default:
			}
		}
		return
	}
	if msg.What == what.Set {
		q.replaceSet(msg)
		return
	}
	if msg.What == what.Update && msg.Dest == nil {
		return
	}
	if q.replaceOldestSet(msg) {
		return
	}
	overloaded = broadcastOverloadCancels(msg)
	return
}

// replaceOldestSet makes room for a one-shot message without losing another
// one-shot message.
func (q *broadcastQueue) replaceOldestSet(msg wire.Message) bool {
	for i := range q.count {
		if q.msgs[(q.head+i)%len(q.msgs)].What == what.Set {
			for j := i; j < q.count-1; j++ {
				q.msgs[(q.head+j)%len(q.msgs)] = q.msgs[(q.head+j+1)%len(q.msgs)]
			}
			q.msgs[(q.head+q.count-1)%len(q.msgs)] = msg
			return true
		}
	}
	return false
}

// replaceSet removes pending writes to the same destination and path, then
// appends the latest frame so it stays ordered after other pending messages.
func (q *broadcastQueue) replaceSet(msg wire.Message) {
	path, _, ok := strings.Cut(msg.Data, "=")
	if !ok {
		return
	}
	// ponytail: scan only on overflow; index paths if large queues make this costly.
	write := 0
	for read := range q.count {
		old := q.msgs[(q.head+read)%len(q.msgs)]
		if old.What == what.Set && sameBroadcastDest(old.Dest, msg.Dest) {
			if oldPath, _, found := strings.Cut(old.Data, "="); found && oldPath == path {
				continue
			}
		}
		q.msgs[(q.head+write)%len(q.msgs)] = old
		write++
	}
	if write < q.count {
		q.msgs[(q.head+write)%len(q.msgs)] = msg
		for clearIndex := write + 1; clearIndex < q.count; clearIndex++ {
			q.msgs[(q.head+clearIndex)%len(q.msgs)] = wire.Message{}
		}
		q.count = write + 1
	}
}

func broadcastOverloadCancels(msg wire.Message) bool {
	return msg.What != what.Set && (msg.What != what.Update || msg.Dest != nil)
}

func sameBroadcastDest(a, b any) bool {
	if aTags, ok := a.([]any); ok {
		bTags, ok := b.([]any)
		if !ok || len(aTags) != len(bTags) {
			return false
		}
		for i := range aTags {
			if aTags[i] != bTags[i] {
				return false
			}
		}
		return true
	}
	if _, ok := b.([]any); ok {
		return false
	}
	return a == b
}

func (q *broadcastQueue) pop() (msg wire.Message, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.count > 0 {
		msg = q.msgs[q.head]
		q.msgs[q.head] = wire.Message{}
		q.head = (q.head + 1) % len(q.msgs)
		q.count--
		ok = true
		if q.count > 0 && !q.closed {
			select {
			case q.ready <- struct{}{}:
			default:
			}
		}
	}
	return
}

func (q *broadcastQueue) close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.ready)
	}
	q.mu.Unlock()
}
