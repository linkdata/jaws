package jaws

import (
	"fmt"
	"sync"
)

const maxQueuedLogs = 4096

type queuedLog struct {
	logger  Logger
	err     error
	dropped uint64
	next    *queuedLog
}

// loggerQueue decouples Logger.Error latency from producers with a bounded FIFO.
// Its mutex is a leaf in the lock hierarchy, and its single consumer releases
// it before invoking Logger.Error.
type loggerQueue struct {
	mu        sync.Mutex
	ready     *sync.Cond
	head      *queuedLog
	tail      *queuedLog
	depth     int
	dropEntry *queuedLog
	doneCh    chan struct{}
	closed    bool
}

func newLoggerQueue() *loggerQueue {
	q := &loggerQueue{doneCh: make(chan struct{})}
	q.ready = sync.NewCond(&q.mu)
	go q.run()
	return q
}

func (q *loggerQueue) enqueue(logger Logger, err error) {
	if q == nil || logger == nil || err == nil {
		return
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	if q.depth >= maxQueuedLogs {
		if q.dropEntry == nil {
			q.dropEntry = &queuedLog{logger: logger}
			q.tail.next = q.dropEntry
			q.tail = q.dropEntry
		}
		q.dropEntry.dropped++
		q.mu.Unlock()
		return
	}
	entry := &queuedLog{logger: logger, err: err}
	if q.tail == nil {
		q.head = entry
	} else {
		q.tail.next = entry
	}
	q.tail = entry
	q.depth++
	q.ready.Signal()
	q.mu.Unlock()
}

func (q *loggerQueue) close() {
	if q != nil {
		q.mu.Lock()
		q.closed = true
		q.ready.Signal()
		q.mu.Unlock()
	}
}

func (q *loggerQueue) pop() (entry *queuedLog) {
	q.mu.Lock()
	for q.head == nil && !q.closed {
		q.ready.Wait()
	}
	if entry = q.head; entry != nil {
		q.head = entry.next
		entry.next = nil
		if entry == q.dropEntry {
			entry.err = fmt.Errorf("jaws: %d diagnostics dropped", entry.dropped)
			q.dropEntry = nil
		} else {
			q.depth--
		}
		if q.head == nil {
			q.tail = nil
		}
	}
	q.mu.Unlock()
	return
}

func (q *loggerQueue) run() {
	defer close(q.doneCh)
	for {
		entry := q.pop()
		if entry == nil {
			return
		}
		entry.dispatch()
	}
}

func (entry *queuedLog) dispatch() {
	// A logger panic must not stop delivery of later entries.
	defer func() { _ = recover() }()
	entry.logger.Error("jaws", "err", entry.err)
}
