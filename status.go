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

type statusSample struct {
	enabled              uint32
	activeRequests       uint64
	pendingRequests      uint64
	sessions             uint64
	activeSessions       uint64
	errors               uint64
	registeredRequestGen uint64
	acceptedWebSocketGen uint64
}

// ActiveRequestCountTag returns this instance's active-Request count tag.
//
// Use it with the active result from [Jaws.RequestCounts]. The tag is stable for
// the Jaws lifetime and unique to this instance and metric. Select
// [StatusMetricActiveRequests] to have maintenance dirty it when its sampled
// count changes and at the next sample after a WebSocket connection is accepted.
func (jw *Jaws) ActiveRequestCountTag() any {
	return &jw.statusTags.activeRequests
}

// PendingRequestCountTag returns this instance's pending-Request count tag.
//
// Use it with [Jaws.Pending]. The tag is stable for the Jaws lifetime and unique
// to this instance and metric. Select [StatusMetricPendingRequests] to have
// maintenance dirty it when its sampled count changes and at the next sample
// after a Request is registered or a WebSocket connection is accepted.
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
// have maintenance dirty it when its sampled count changes and at the next sample
// after a WebSocket connection is accepted.
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
// maintenance dirty it when the count changes.
func (jw *Jaws) ErrorCountTag() any {
	return &jw.statusTags.errors
}

func statusCount(n int) uint64 {
	return uint64(n) // #nosec G115 -- collection-derived status counts are non-negative.
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

func (jw *Jaws) statusMetricLocked(metric uint32) (tag *statusTag, value uint64, previous *uint64) {
	switch metric {
	case StatusMetricActiveRequests:
		tag = &jw.statusTags.activeRequests
		value = statusCount(jw.activeRequestCountLocked())
		previous = &jw.statusSample.activeRequests
	case StatusMetricPendingRequests:
		tag = &jw.statusTags.pendingRequests
		value = statusCount(jw.pendingRequestCountLocked())
		previous = &jw.statusSample.pendingRequests
	case StatusMetricSessions:
		tag = &jw.statusTags.sessions
		value = statusCount(len(jw.sessions))
		previous = &jw.statusSample.sessions
	case StatusMetricActiveSessions:
		tag = &jw.statusTags.activeSessions
		value = statusCount(jw.activeSessionCountLocked())
		previous = &jw.statusSample.activeSessions
	case StatusMetricErrors:
		tag = &jw.statusTags.errors
		value = jw.reportedErrors.Load()
		previous = &jw.statusSample.errors
	}
	return
}

func (jw *Jaws) updateStatusLocked() {
	enabled := jw.StatusMetrics.Load() & StatusMetricAll
	if enabled == 0 {
		jw.statusSample.enabled = 0
		return
	}
	forceDirty := enabled &^ jw.statusSample.enabled
	// Lifecycle generations detect changes that return a gauge to its previous sample.
	if jw.registeredRequestGen != jw.statusSample.registeredRequestGen {
		forceDirty |= StatusMetricPendingRequests
	}
	if jw.acceptedWebSocketGen != jw.statusSample.acceptedWebSocketGen {
		forceDirty |= StatusMetricActiveRequests | StatusMetricPendingRequests | StatusMetricActiveSessions
	}
	for metric := StatusMetricActiveRequests; metric != 0 && metric <= StatusMetricAll; metric <<= 1 {
		if enabled&metric != 0 {
			tag, value, previous := jw.statusMetricLocked(metric)
			if forceDirty&metric != 0 || value != *previous {
				jw.addDirtLocked(tag)
			}
			*previous = value
		}
	}
	jw.statusSample.enabled = enabled
	jw.statusSample.registeredRequestGen = jw.registeredRequestGen
	jw.statusSample.acceptedWebSocketGen = jw.acceptedWebSocketGen
}
