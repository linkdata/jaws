package jaws

const (
	// StatusMetricActiveRequests enables dirtying [Jaws.ActiveRequestCountTag].
	StatusMetricActiveRequests uint32 = 1 << iota
	// StatusMetricPendingRequests enables dirtying [Jaws.PendingRequestCountTag].
	StatusMetricPendingRequests
	// StatusMetricSessions enables dirtying [Jaws.SessionCountTag].
	StatusMetricSessions
	// StatusMetricActiveSessions enables dirtying [Jaws.ActiveSessionCountTag].
	StatusMetricActiveSessions
	// StatusMetricErrors enables dirtying [Jaws.ErrorCountTag].
	StatusMetricErrors
	// StatusMetricAll selects every status metric.
	StatusMetricAll = StatusMetricActiveRequests |
		StatusMetricPendingRequests |
		StatusMetricSessions |
		StatusMetricActiveSessions |
		StatusMetricErrors
)

// statusTag is non-zero-sized so pointers to distinct fields cannot share an
// address. Its value is immaterial; pointer identity is the dependency key.
type statusTag uint8

type statusTags struct {
	activeRequests  statusTag
	pendingRequests statusTag
	sessions        statusTag
	activeSessions  statusTag
	errors          statusTag
}

type statusSample struct {
	enabled         uint32
	activeRequests  int
	pendingRequests int
	sessions        int
	activeSessions  int
	errors          uint64
}

// ActiveRequestCountTag returns this instance's active-Request count tag.
//
// Use it with the active result from [Jaws.RequestCounts]. The tag is stable for
// the Jaws lifetime and unique to this instance and metric. Select
// [StatusMetricActiveRequests] to have maintenance dirty it when the count changes.
func (jw *Jaws) ActiveRequestCountTag() any {
	return &jw.statusTags.activeRequests
}

// PendingRequestCountTag returns this instance's pending-Request count tag.
//
// Use it with [Jaws.Pending]. The tag is stable for the Jaws lifetime and unique
// to this instance and metric. Select [StatusMetricPendingRequests] to have
// maintenance dirty it when the count changes.
func (jw *Jaws) PendingRequestCountTag() any {
	return &jw.statusTags.pendingRequests
}

// SessionCountTag returns this instance's registered-Session count tag.
//
// Use it with [Jaws.SessionCount]. The tag is stable for the Jaws lifetime and
// unique to this instance and metric. Select [StatusMetricSessions] to have
// maintenance dirty it when the count changes.
func (jw *Jaws) SessionCountTag() any {
	return &jw.statusTags.sessions
}

// ActiveSessionCount returns the active Session count.
//
// It counts registered Sessions attached to at least one Request whose
// [Request.ServeHTTP] loop is running. A Session shared by several running Requests
// counts once. A Session retained only for its disconnect grace period is inactive.
// ActiveSessionCount is safe for concurrent use.
func (jw *Jaws) ActiveSessionCount() (n int) {
	jw.mu.RLock()
	n = jw.activeSessionCountLocked()
	jw.mu.RUnlock()
	return
}

// ActiveSessionCountTag returns this instance's active-Session count tag.
//
// Use it with [Jaws.ActiveSessionCount]. The tag is stable for the Jaws lifetime
// and unique to this instance and metric. Select [StatusMetricActiveSessions] to
// have maintenance dirty it when the count changes.
func (jw *Jaws) ActiveSessionCountTag() any {
	return &jw.statusTags.activeSessions
}

// ErrorCount returns the number of errors reported to this instance.
//
// Every non-nil error passed to [Jaws.Log] or [Jaws.MustLog] counts, including
// calls through [Request.Log] and [Request.MustLog], calls without a Logger, and
// calls after shutdown. Logger delivery, latency, and panics do not affect the
// count. ErrorCount is safe for concurrent use. [StatusMetricErrors] controls tag
// updates, not counting.
func (jw *Jaws) ErrorCount() uint64 {
	return jw.reportedErrors.Load()
}

// ErrorCountTag returns this instance's reported-error count tag.
//
// Use it with [Jaws.ErrorCount]. The tag is stable for the Jaws lifetime and
// unique to this instance and metric. Select [StatusMetricErrors] to have
// maintenance dirty it when the count changes.
func (jw *Jaws) ErrorCountTag() any {
	return &jw.statusTags.errors
}

func (jw *Jaws) activeRequestCountLocked() (n int) {
	for _, rq := range jw.requests {
		if rq != nil && rq.loadState() == reqRunning {
			n++
		}
	}
	return
}

func (jw *Jaws) pendingRequestCountLocked() (n int) {
	for _, pending := range jw.pending {
		n += len(pending)
	}
	return
}

func (jw *Jaws) activeSessionCountLocked() (n int) {
	for _, sess := range jw.sessions {
		sess.mu.RLock()
		for _, rq := range sess.requests {
			if rq.loadState() == reqRunning {
				n++
				break
			}
		}
		sess.mu.RUnlock()
	}
	return
}
