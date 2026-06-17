package jaws

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/key"
)

// TestRequest_JawsKeyReadsAreLockedDuringRecycle verifies that the request-key
// readers used while the application renders the initial HTML page
// (JawsKeyString, String, HeadHTML and TailHTML) read rq.JawsKey under rq.mu, so
// they do not race the rq.mu-guarded writes to rq.JawsKey that clearLocked and
// getRequestLocked perform when a still-pending request is recycled (for example
// by limitPendingRequestsLocked evicting it). Run with -race.
func TestRequest_JawsKeyReadsAreLockedDuringRecycle(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: mimic clearLocked / getRequestLocked, which assign rq.JawsKey while
	// holding rq.mu.
	go func() {
		defer wg.Done()
		for i := range iterations {
			rq.mu.Lock()
			rq.JawsKey = key.Key(uint64(i) + 1)
			rq.mu.Unlock()
		}
	}()

	// Reader: the lock-free render-path readers that previously read rq.JawsKey
	// without holding rq.mu.
	go func() {
		defer wg.Done()
		for range iterations {
			_ = rq.JawsKeyString()
			_ = rq.String()
			_ = rq.HeadHTML(io.Discard)
			_ = rq.TailHTML(io.Discard)
		}
	}()

	wg.Wait()
}

// TestServe_MarksRequestRunningSoMaintenanceSkips verifies that TestServe marks
// the request running before driving rq.process, mirroring ServeHTTP/startServe.
// The maintenance pass recycles only not-running requests (clearing their
// elements via clearLocked), so a request whose process loop is live must report
// running and must survive a maintenance pass that would otherwise expire it.
func TestServe_MarksRequestRunningSoMaintenanceSkips(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	go jw.Serve()

	tr := NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()

	<-tr.ReadyCh
	if !tr.running.Load() {
		t.Fatal("TestServe must mark the request running so maintenance cannot recycle it mid-process")
	}

	// Make the request look long-idle, then run a maintenance pass directly. A
	// running request must not be recycled; before the fix it would be removed
	// from jw.requests and its elements cleared while process is still using them.
	tr.mu.Lock()
	tr.lastWrite = time.Now().Add(-time.Hour)
	tr.mu.Unlock()
	jw.maintenance(time.Millisecond)

	if got := jw.RequestCount(); got != 1 {
		t.Fatalf("running request was recycled by maintenance: RequestCount() = %d, want 1", got)
	}
}
