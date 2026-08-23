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
)

// StatusMetricAll selects every status metric.
const StatusMetricAll = StatusMetricActiveRequests |
	StatusMetricPendingRequests |
	StatusMetricSessions |
	StatusMetricActiveSessions |
	StatusMetricErrors

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

// ActiveRequestCountTag returns this instance's active-Request count tag.
//
// Use it with the active result from [Jaws.RequestCounts]. The tag is stable for
// the Jaws lifetime and unique to this instance and metric. Select
// [StatusMetricActiveRequests] to have maintenance dirty it; [Jaws.StatusMetrics]
// defines when.
func (jw *Jaws) ActiveRequestCountTag() any {
	return &jw.statusTags.activeRequests
}

// PendingRequestCountTag returns this instance's pending-Request count tag.
//
// Use it with [Jaws.Pending]. The tag is stable for the Jaws lifetime and unique
// to this instance and metric. Select [StatusMetricPendingRequests] to have
// maintenance dirty it; [Jaws.StatusMetrics] defines when.
func (jw *Jaws) PendingRequestCountTag() any {
	return &jw.statusTags.pendingRequests
}

// SessionCountTag returns this instance's registered-Session count tag.
//
// Use it with [Jaws.SessionCount]. The tag is stable for the Jaws lifetime and
// unique to this instance and metric. Select [StatusMetricSessions] to have
// maintenance dirty it; [Jaws.StatusMetrics] defines when.
func (jw *Jaws) SessionCountTag() any {
	return &jw.statusTags.sessions
}

// ActiveSessionCount returns the active Session count.
//
// It counts registered Sessions attached to at least one Request whose
// [Request.ServeHTTP] loop is running. A Session shared by several running Requests
// counts once. A Session retained only for its disconnect grace period is inactive.
// It is safe for concurrent use.
func (jw *Jaws) ActiveSessionCount() (n int) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	n = jw.activeSessionCountLocked()
	return
}

// ActiveSessionCountTag returns this instance's active-Session count tag.
//
// Use it with [Jaws.ActiveSessionCount]. The tag is stable for the Jaws lifetime
// and unique to this instance and metric. Select [StatusMetricActiveSessions] to
// have maintenance dirty it; [Jaws.StatusMetrics] defines when.
func (jw *Jaws) ActiveSessionCountTag() any {
	return &jw.statusTags.activeSessions
}

// ErrorCount returns the number of errors reported to this instance.
//
// A non-nil error reported through this instance's [Jaws.Log] or [Jaws.MustLog]
// increments the count, including calls through [Request.Log] or [Request.MustLog].
// Counting continues without a Logger and after [Jaws.Done] closes; Logger
// delivery, latency, and panics do not affect it.
//
// ErrorCount is safe for concurrent use. [StatusMetricErrors] controls tag
// updates, not counting.
func (jw *Jaws) ErrorCount() uint64 {
	return jw.reportedErrors.Load()
}

// ErrorCountTag returns this instance's reported-error count tag.
//
// Use it with [Jaws.ErrorCount]. The tag is stable for the Jaws lifetime and
// unique to this instance and metric. Select [StatusMetricErrors] to have
// maintenance dirty it; [Jaws.StatusMetrics] defines when.
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
			if rq != nil && rq.loadState() == reqRunning {
				n++
				break
			}
		}
		sess.mu.RUnlock()
	}
	return
}

func (jw *Jaws) statusTagForMetric(metric uint32) (tag *statusTag) {
	switch metric {
	case StatusMetricActiveRequests:
		tag = &jw.statusTags.activeRequests
	case StatusMetricPendingRequests:
		tag = &jw.statusTags.pendingRequests
	case StatusMetricSessions:
		tag = &jw.statusTags.sessions
	case StatusMetricActiveSessions:
		tag = &jw.statusTags.activeSessions
	case StatusMetricErrors:
		tag = &jw.statusTags.errors
	}
	return
}

// markStatusDirty records coalescible changes for the next maintenance pass.
func (jw *Jaws) markStatusDirty(metrics uint32) {
	jw.statusDirty.Or(metrics)
}

func (jw *Jaws) updateStatusLocked() {
	enabled := jw.StatusMetrics.Load() & StatusMetricAll
	dirty := enabled & (jw.statusDirty.Swap(0) | (enabled &^ jw.enabledStatusMetrics))
	for metric := StatusMetricActiveRequests; metric != 0 && metric <= StatusMetricAll; metric <<= 1 {
		if dirty&metric != 0 {
			jw.addDirtLocked(jw.statusTagForMetric(metric))
		}
	}
	jw.enabledStatusMetrics = enabled
}
