package jaws

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/secureheaders"
	"github.com/linkdata/staticserve"
)

type testBroadcastTagGetter struct{}

func (testBroadcastTagGetter) JawsGetTag(tag.Context) any {
	return tag.Tag("expanded")
}

type mutatingTemplateLookuper struct {
	jw *Jaws
}

func (tl mutatingTemplateLookuper) Lookup(string) *template.Template {
	_ = tl.jw.AddTemplateLookuper(testTemplateLookuper{})
	return nil
}

type testTemplateLookuper struct{}

func (testTemplateLookuper) Lookup(string) *template.Template {
	return nil
}

type captureErrorLogger struct {
	err error
}

func (l *captureErrorLogger) Info(string, ...any) {}
func (l *captureErrorLogger) Warn(string, ...any) {}
func (l *captureErrorLogger) Error(_ string, args ...any) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			if err, ok := args[i+1].(error); ok {
				l.err = err
			}
		}
	}
}

func TestMustLog_PanicsWithoutLogger(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	defer func() {
		if recover() == nil {
			t.Error("MustLog with no Logger must panic")
		}
	}()
	jw.MustLog(errors.New("boom"))
}

func TestNew_DefaultWebSocketPingInterval(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if got, want := jw.WebSocketPingInterval, DefaultWebSocketPingInterval; got != want {
		t.Fatalf("WebSocketPingInterval = %v, want %v", got, want)
	}
	if got, want := jw.MaxPendingRequestsPerIP, DefaultMaxPendingRequestsPerIP; got != want {
		t.Fatalf("MaxPendingRequestsPerIP = %v, want %v", got, want)
	}
}

func TestJaws_MaxPendingRequestsPerIPDisabled(t *testing.T) {
	for _, limit := range []int{0, -1} {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		defer jw.Close()
		jw.MaxPendingRequestsPerIP = limit

		jw.NewRequest(newPendingLimitRequest("192.0.2.1:1000"))
		jw.NewRequest(newPendingLimitRequest("192.0.2.1:1001"))
		jw.NewRequest(newPendingLimitRequest("192.0.2.1:1002"))

		if got := jw.Pending(); got != 3 {
			t.Fatalf("Pending() with limit %d = %d, want 3", limit, got)
		}
		if got := jw.RequestCount(); got != 3 {
			t.Fatalf("RequestCount() with limit %d = %d, want 3", limit, got)
		}
	}
}

func TestJaws_MaxPendingRequestsPerIPEvictsOldestPending(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 2

	oldReq := newPendingLimitRequest("192.0.2.1:1000")
	oldRq := jw.NewRequest(oldReq)
	oldKey := oldRq.JawsKey
	setPendingLimitLastWrite(t, oldRq, 7200)

	midReq := newPendingLimitRequest("192.0.2.1:1001")
	midRq := jw.NewRequest(midReq)
	midKey := midRq.JawsKey
	setPendingLimitLastWrite(t, midRq, 3600)

	newReq := newPendingLimitRequest("192.0.2.1:1002")
	newRq := jw.NewRequest(newReq)
	newKey := newRq.JawsKey

	if got := jw.Pending(); got != 2 {
		t.Fatalf("Pending() = %d, want 2", got)
	}
	if got := jw.RequestCount(); got != 2 {
		t.Fatalf("RequestCount() = %d, want 2", got)
	}
	if claimed := jw.UseRequest(oldKey, oldReq); claimed != nil {
		t.Fatalf("evicted request claimed as %v", claimed)
	}
	if claimed := jw.UseRequest(midKey, midReq); claimed != midRq {
		t.Fatalf("middle request claim = %v, want %v", claimed, midRq)
	}
	if claimed := jw.UseRequest(newKey, newReq); claimed != newRq {
		t.Fatalf("new request claim = %v, want %v", claimed, newRq)
	}
}

// reentrantLogger re-enters the Jaws instance while logging (via RequestCount,
// which takes jw.mu.RLock), and records the logged error. If the framework ever
// invokes the logger while holding jw.mu, this deadlocks.
type reentrantLogger struct {
	jw     *Jaws
	logged chan error
}

func (l reentrantLogger) Info(string, ...any) {}
func (l reentrantLogger) Warn(string, ...any) {}
func (l reentrantLogger) Error(_ string, args ...any) {
	_ = l.jw.RequestCount() // re-enter Jaws under jw.mu.RLock
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			if err, ok := args[i+1].(error); ok {
				select {
				case l.logged <- err:
				default:
				}
			}
		}
	}
}

// TestJaws_MaintenanceLogsCancelOutsideLock verifies the maintenance pass logs a
// request-cancellation cause AFTER releasing jw.mu, so a user Logger that re-enters
// the Jaws instance does not deadlock the Serve loop. Before the fix the logger ran
// while jw.mu was held for writing, which would deadlock on any re-entrant lock.
func TestJaws_MaintenanceLogsCancelOutsideLock(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logged := make(chan error, 1)
	jw.Logger = reentrantLogger{jw: jw, logged: logged}

	// A pending request idle long enough for the maintenance pass to recycle it.
	rq := jw.NewRequest(newPendingLimitRequest("192.0.2.1:1000"))
	setPendingLimitLastWrite(t, rq, 3600)

	done := make(chan struct{})
	go func() {
		jw.maintenance(time.Second)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("maintenance deadlocked: user Logger re-entered Jaws while jw.mu was held")
	}
	select {
	case e := <-logged:
		if !errors.Is(e, ErrNoWebSocketRequest) {
			t.Fatalf("logged cause = %v, want ErrNoWebSocketRequest", e)
		}
	default:
		t.Fatal("expected the cancellation cause to be logged")
	}
}

// TestJaws_MaxPendingRequestsPerIPSparesRenderingRequest verifies that the pending
// cap does not evict a Request that is still rendering its initial HTML. The oldest
// pending Request would normally be evicted first, but evicting one whose render is
// in flight on another goroutine would recycle it underneath that goroutine,
// contaminating a later reuse and leaking its key. The cap must instead evict the
// next-oldest idle Request, leaving the rendering one claimable.
func TestJaws_MaxPendingRequestsPerIPSparesRenderingRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 2

	renderingReq := newPendingLimitRequest("192.0.2.1:1000")
	renderingRq := jw.NewRequest(renderingReq)
	renderingKey := renderingRq.JawsKey
	// Simulate an in-flight initial render on the oldest pending Request: a fresh
	// write timestamp. maintenanceInterval is zero here, so the spare window uses
	// the DefaultUpdateInterval floor.
	renderingRq.MarkWritten()

	idleReq := newPendingLimitRequest("192.0.2.1:1001")
	idleRq := jw.NewRequest(idleReq)
	idleKey := idleRq.JawsKey
	// Age the idle Request well past the spare window so it is the eviction victim.
	setPendingLimitLastWrite(t, idleRq, 3600)

	// Creating a third same-IP Request trips the cap (pending == 2).
	newReq := newPendingLimitRequest("192.0.2.1:1002")
	newRq := jw.NewRequest(newReq)
	newKey := newRq.JawsKey

	// The rendering Request must survive; the idle one is the one evicted.
	if claimed := jw.UseRequest(renderingKey, renderingReq); claimed != renderingRq {
		t.Fatalf("rendering request claim = %v, want it to survive eviction", claimed)
	}
	if claimed := jw.UseRequest(idleKey, idleReq); claimed != nil {
		t.Fatalf("idle request should have been evicted, got %v", claimed)
	}
	if claimed := jw.UseRequest(newKey, newReq); claimed != newRq {
		t.Fatalf("new request claim = %v, want %v", claimed, newRq)
	}
}

// TestJaws_MaxPendingRequestsPerIPSparesRecentlyRenderedRequest verifies that with a
// configured maintenanceInterval the pending cap spares a Request written within the
// 2*maintenanceInterval recency window and falls through to evict a genuinely idle one
// instead.
func TestJaws_MaxPendingRequestsPerIPSparesRecentlyRenderedRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 2
	// Pretend the Serve loop is running with a one-minute maintenance interval so the
	// recency window is 2*maintenanceInterval (it falls back to DefaultUpdateInterval
	// until ServeWithTimeout sets it).
	jw.mu.Lock()
	jw.maintenanceInterval = time.Minute
	jw.mu.Unlock()

	// The oldest pending Request wrote recently (its render is still in flight).
	renderingReq := newPendingLimitRequest("192.0.2.1:1000")
	renderingRq := jw.NewRequest(renderingReq)
	renderingKey := renderingRq.JawsKey
	renderingRq.MarkWritten()

	// A genuinely idle Request that rendered long ago is the correct eviction victim.
	idleReq := newPendingLimitRequest("192.0.2.1:1001")
	idleRq := jw.NewRequest(idleReq)
	idleKey := idleRq.JawsKey
	setPendingLimitLastWrite(t, idleRq, 3600)

	// A third same-IP Request trips the cap (pending == 2).
	newReq := newPendingLimitRequest("192.0.2.1:1002")
	newRq := jw.NewRequest(newReq)
	newKey := newRq.JawsKey

	// The recently-rendered Request must survive despite its cleared flag; the idle one
	// is evicted instead.
	if claimed := jw.UseRequest(renderingKey, renderingReq); claimed != renderingRq {
		t.Fatalf("recently-rendered request claim = %v, want it to survive eviction", claimed)
	}
	if claimed := jw.UseRequest(idleKey, idleReq); claimed != nil {
		t.Fatalf("idle request should have been evicted, got %v", claimed)
	}
	if claimed := jw.UseRequest(newKey, newReq); claimed != newRq {
		t.Fatalf("new request claim = %v, want %v", claimed, newRq)
	}
}

// TestJaws_MaxPendingRequestsPerIPSparesStalledLiveRender proves the eviction decision
// tracks the actual last write, not a value sampled by the maintenance pass: a render
// that wrote once and then stalled (no further writes) is still spared while that write
// is within 2*maintenanceInterval, even across a maintenance pass — closing the gap
// where clearing a flag on a tick could expose a live render to recycling.
func TestJaws_MaxPendingRequestsPerIPSparesStalledLiveRender(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 2
	jw.mu.Lock()
	jw.maintenanceInterval = time.Minute
	jw.mu.Unlock()

	// Oldest pending: a single write, then silence. Its timestamp stays fresh.
	stalledReq := newPendingLimitRequest("192.0.2.1:1000")
	stalledRq := jw.NewRequest(stalledReq)
	stalledKey := stalledRq.JawsKey
	stalledRq.MarkWritten()

	// A maintenance pass must not disturb the write timestamp (generous timeout so the
	// stalled render is not idle-expired). Under a flag-cleared-by-tick scheme this is
	// where protection could be lost.
	jw.maintenance(time.Hour)

	// A genuinely idle sibling is the correct victim.
	idleReq := newPendingLimitRequest("192.0.2.1:1001")
	idleRq := jw.NewRequest(idleReq)
	idleKey := idleRq.JawsKey
	setPendingLimitLastWrite(t, idleRq, 3600)

	// A third same-IP Request trips the cap (pending == 2).
	newReq := newPendingLimitRequest("192.0.2.1:1002")
	newRq := jw.NewRequest(newReq)
	newKey := newRq.JawsKey

	if claimed := jw.UseRequest(stalledKey, stalledReq); claimed != stalledRq {
		t.Fatalf("stalled-but-recently-written render claim = %v, want it to survive eviction", claimed)
	}
	if claimed := jw.UseRequest(idleKey, idleReq); claimed != nil {
		t.Fatalf("idle request should have been evicted, got %v", claimed)
	}
	if claimed := jw.UseRequest(newKey, newReq); claimed != newRq {
		t.Fatalf("new request claim = %v, want %v", claimed, newRq)
	}
}

func TestJaws_MaxPendingRequestsPerIPSparesFutureWriteTimestamp(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 1

	renderingReq := newPendingLimitRequest("192.0.2.1:1000")
	renderingRq := jw.NewRequest(renderingReq)
	renderingKey := renderingRq.JawsKey
	nowSeconds := jw.runtimeSeconds.Load()
	renderingRq.lastWriteSeconds.Store(nowSeconds + 1)

	newReq := newPendingLimitRequest("192.0.2.1:1001")
	newRq := jw.NewRequest(newReq)
	newKey := newRq.JawsKey

	if got := jw.Pending(); got != 2 {
		t.Fatalf("Pending() = %d, want 2 (cap overshoots while render timestamp is newer than scan timestamp)", got)
	}
	if claimed := jw.UseRequest(renderingKey, renderingReq); claimed != renderingRq {
		t.Fatalf("future-written render claim = %v, want it to survive eviction", claimed)
	}
	if claimed := jw.UseRequest(newKey, newReq); claimed != newRq {
		t.Fatalf("new request claim = %v, want %v", claimed, newRq)
	}
}

// TestJaws_MaxPendingRequestsPerIPOvershootsWhenAllRendering verifies that when
// every pending request for an IP is mid-render, the cap is not enforced by
// recycling one of them: oldestEvictablePendingLocked finds no idle victim, so the
// cap is allowed a brief, self-correcting overshoot rather than corrupting an
// in-flight render.
func TestJaws_MaxPendingRequestsPerIPOvershootsWhenAllRendering(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 1

	rq1 := jw.NewRequest(newPendingLimitRequest("192.0.2.1:1000"))
	rq1Key := rq1.JawsKey
	rq1.MarkWritten()

	// A second same-IP request trips the cap, but the only pending request is
	// rendering, so nothing is evicted and the cap overshoots to 2.
	rq2 := jw.NewRequest(newPendingLimitRequest("192.0.2.1:1001"))
	if rq2 == nil {
		t.Fatal("expected the new request to be created despite the cap")
	}
	if got := jw.Pending(); got != 2 {
		t.Fatalf("Pending() = %d, want 2 (cap overshoots while all pending render)", got)
	}
	if rq1.JawsKey == 0 || rq1.JawsKey != rq1Key {
		t.Fatal("the rendering request must not be recycled by the cap")
	}
}

func TestJaws_MaxPendingRequestsPerIPKeepsDifferentIPs(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 1

	reqA := newPendingLimitRequest("192.0.2.1:1000")
	rqA := jw.NewRequest(reqA)
	reqB := newPendingLimitRequest("198.51.100.1:1000")
	rqB := jw.NewRequest(reqB)

	if got := jw.Pending(); got != 2 {
		t.Fatalf("Pending() = %d, want 2", got)
	}
	if claimed := jw.UseRequest(rqA.JawsKey, reqA); claimed != rqA {
		t.Fatalf("request A claim = %v, want %v", claimed, rqA)
	}
	if claimed := jw.UseRequest(rqB.JawsKey, reqB); claimed != rqB {
		t.Fatalf("request B claim = %v, want %v", claimed, rqB)
	}
}

func TestJaws_MaxPendingRequestsPerIPIgnoresClaimedRequests(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 1

	claimedReq := newPendingLimitRequest("192.0.2.1:1000")
	claimedRq := jw.NewRequest(claimedReq)
	if claimed := jw.UseRequest(claimedRq.JawsKey, claimedReq); claimed != claimedRq {
		t.Fatalf("claimed request claim = %v, want %v", claimed, claimedRq)
	}
	if got := jw.Pending(); got != 0 {
		t.Fatalf("Pending() after claim = %d, want 0", got)
	}

	pendingReq := newPendingLimitRequest("192.0.2.1:1001")
	pendingRq := jw.NewRequest(pendingReq)

	total, active := jw.RequestCounts()
	if total != 2 || active != 0 {
		t.Fatalf("RequestCounts() = %d, %d, want 2, 0", total, active)
	}
	if got := jw.Pending(); got != 1 {
		t.Fatalf("Pending() = %d, want 1", got)
	}
	jw.mu.RLock()
	stillPresent := jw.requests[claimedRq.JawsKey] == claimedRq
	jw.mu.RUnlock()
	if !stillPresent {
		t.Fatal("claimed request was evicted")
	}
	if claimed := jw.UseRequest(pendingRq.JawsKey, pendingReq); claimed != pendingRq {
		t.Fatalf("pending request claim = %v, want %v", claimed, pendingRq)
	}
}

func TestJaws_MaxPendingRequestsPerIPKeepsLoopbackAddressesSeparate(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 1

	oldReq := newPendingLimitRequest("127.0.0.1:1000")
	oldRq := jw.NewRequest(oldReq)
	setPendingLimitLastWrite(t, oldRq, 3600)

	newReq := newPendingLimitRequest("[::1]:1000")
	newRq := jw.NewRequest(newReq)

	if got := jw.Pending(); got != 2 {
		t.Fatalf("Pending() = %d, want 2", got)
	}
	if claimed := jw.UseRequest(oldRq.JawsKey, oldReq); claimed != oldRq {
		t.Fatalf("first loopback request claim = %v, want %v", claimed, oldRq)
	}
	if claimed := jw.UseRequest(newRq.JawsKey, newReq); claimed != newRq {
		t.Fatalf("new loopback request claim = %v, want %v", claimed, newRq)
	}
}

func TestJaws_LookupTemplateDoesNotHoldJawsLockDuringLookup(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err = jw.AddTemplateLookuper(mutatingTemplateLookuper{jw: jw}); err != nil {
		t.Fatal(err)
	}

	done := make(chan any, 1)
	go func() {
		defer func() {
			done <- recover()
		}()
		_ = jw.LookupTemplate("test")
	}()

	select {
	case recovered := <-done:
		if recovered != nil {
			t.Fatalf("LookupTemplate panicked while a lookuper mutated template lookupers: %v", recovered)
		}
		jw.Close()
	case <-time.After(100 * time.Millisecond):
		t.Fatal("LookupTemplate deadlocked while a lookuper mutated template lookupers")
	}
}

func TestJaws_MaxPendingRequestsPerIPEvictionCause(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger
	jw.MaxPendingRequestsPerIP = 1

	oldReq := newPendingLimitRequest("192.0.2.1:1000")
	oldRq := jw.NewRequest(oldReq)
	setPendingLimitLastWrite(t, oldRq, 3600)
	jw.NewRequest(newPendingLimitRequest("192.0.2.1:1001"))

	if logger.err == nil {
		t.Fatal("expected eviction error to be logged")
	}
	if !errors.Is(logger.err, ErrRequestCancelled) {
		t.Fatalf("logged error = %v, want ErrRequestCancelled", logger.err)
	}
	if !errors.Is(logger.err, ErrTooManyPendingRequests) {
		t.Fatalf("logged error = %v, want ErrTooManyPendingRequests", logger.err)
	}
	var limitErr errTooManyPendingRequests
	if !errors.As(logger.err, &limitErr) {
		t.Fatalf("logged error = %T, want errTooManyPendingRequests", logger.err)
	}
	if limitErr.Limit != 1 || limitErr.Addr.Compare(parseIP(oldReq.RemoteAddr)) != 0 {
		t.Fatalf("eviction detail = %#v, want limit 1 and IP %v", limitErr, parseIP(oldReq.RemoteAddr))
	}
	if got, want := limitErr.Error(), "too many pending requests from 192.0.2.1 (limit 1)"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

// TestJaws_BroadcastMultiRuntimeNonComparable verifies that broadcasting to a Dest
// slice of two same-typed runtime-non-comparable values is rejected and logged
// rather than panicking the caller. Such values pass tag expansion's static
// comparability check but panic on ==; TagExpand's recover converts that to
// ErrNotUsableAsTag, which Broadcast logs before returning with no tags to send.
func TestJaws_BroadcastMultiRuntimeNonComparable(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	type box struct{ v any }
	dest := []any{box{v: func() {}}, box{v: func() {}}}
	jw.Broadcast(wire.Message{Dest: dest}) // must not panic

	if !errors.Is(logger.err, tag.ErrNotUsableAsTag) {
		t.Fatalf("logged error = %v, want ErrNotUsableAsTag", logger.err)
	}
}

// TestJaws_dropNonComparableTags exercises the Broadcast comparability guard
// directly. TagExpand rejects runtime-non-comparable struct/array tags before
// Broadcast reaches this guard, so the guard is tested with a synthetic tag slice.
func TestJaws_dropNonComparableTags(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.Logger = &captureErrorLogger{}

	// Comparable tags pass through unchanged.
	in := []any{tag.Tag("a"), tag.Tag("b")}
	if got := jw.dropNonComparableTags(in); len(got) != len(in) {
		t.Fatalf("comparable tags dropped: got %v, want %v", got, in)
	}

	// A runtime-non-comparable tag is reported via reportMisuse, which logs and then
	// panics under deadlock.Debug; in production it logs and returns nil.
	bad := []any{[]int{1}}
	if deadlock.Debug {
		func() {
			defer func() {
				switch e := recover().(type) {
				case nil:
					t.Fatal("expected reportMisuse to panic under deadlock.Debug")
				case error:
					if !errors.Is(e, tag.ErrNotComparable) {
						t.Fatalf("panic = %v, want ErrNotComparable", e)
					}
				default:
					t.Fatalf("panic = %v, want error", e)
				}
			}()
			jw.dropNonComparableTags(bad)
		}()
	} else if got := jw.dropNonComparableTags(bad); got != nil {
		t.Fatalf("non-comparable tag: got %v, want nil", got)
	}
}

// TestJaws_setDirtyPanicReleasesLock verifies the documented defer-unlock in
// setDirty: a tag comparable statically but not at runtime panics when used as a
// map key, yet jw.mu is still released so the instance stays usable. Public Dirty
// rejects this tag during expansion, so setDirty is exercised directly with a
// synthetic tag.
func TestJaws_setDirtyPanicReleasesLock(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	// funcTag is statically comparable (a struct with an interface field) but panics
	// when hashed because the field holds a func.
	type funcTag struct{ fn any }

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected setDirty to panic on a runtime-non-comparable tag")
			}
		}()
		jw.setDirty([]any{funcTag{fn: func() {}}})
	}()

	// If the deferred Unlock did not run, this second setDirty would deadlock
	// re-acquiring jw.mu; completing proves the lock was released.
	jw.setDirty([]any{tag.Tag("ok")})
}

func TestJaws_MaxPendingRequestsPerIPMaintenanceRemovesPendingIndex(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxPendingRequestsPerIP = 1

	rq := jw.NewRequest(newPendingLimitRequest("192.0.2.1:1000"))
	setPendingLimitLastWrite(t, rq, 3600)
	jw.maintenance(time.Second)

	if got := jw.Pending(); got != 0 {
		t.Fatalf("Pending() after maintenance = %d, want 0", got)
	}
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("RequestCount() after maintenance = %d, want 0", got)
	}
}

func newPendingLimitRequest(remoteAddr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	return r
}

func setPendingLimitLastWrite(t *testing.T, rq *Request, secondsAgo int32) {
	t.Helper()
	// lastWriteSeconds holds the Jaws.runtimeSeconds value at the last write. Store
	// runtimeSeconds-secondsAgo so the eviction/idle check (runtimeSeconds-lastWrite)
	// reads back as secondsAgo. In tests where Serve has not started, runtimeSeconds
	// is often zero, so old timestamps are represented as negative seconds.
	rq.lastWriteSeconds.Store(rq.Jaws.runtimeSeconds.Load() - secondsAgo)
}

func TestCoverage_GenerateHeadAndConvenienceBroadcasts(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err := jw.GenerateHeadHTML("%zz"); err == nil {
		t.Fatal("expected url parse error")
	}
	if err := jw.GenerateHeadHTML("/favicon.ico", "/app.js"); err != nil {
		t.Fatal(err)
	}

	jw.Reload()
	if msg := nextBroadcast(t, jw); msg.What != what.Reload {
		t.Fatalf("unexpected reload msg %#v", msg)
	}
	jw.Redirect("/next")
	if msg := nextBroadcast(t, jw); msg.What != what.Redirect || msg.Data != "/next" {
		t.Fatalf("unexpected redirect msg %#v", msg)
	}
	jw.Alert("info", "hello")
	if msg := nextBroadcast(t, jw); msg.What != what.Alert || msg.Data != "info\nhello" {
		t.Fatalf("unexpected alert msg %#v", msg)
	}

	jw.SetInner("t", template.HTML("<b>x</b>"))
	if msg := nextBroadcast(t, jw); msg.What != what.Inner || msg.Data != "<b>x</b>" {
		t.Fatalf("unexpected set inner msg %#v", msg)
	}
	jw.SetAttr("t", "k", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.SAttr || msg.Data != "k\nv" {
		t.Fatalf("unexpected set attr msg %#v", msg)
	}
	jw.RemoveAttr("t", "k")
	if msg := nextBroadcast(t, jw); msg.What != what.RAttr || msg.Data != "k" {
		t.Fatalf("unexpected remove attr msg %#v", msg)
	}
	jw.SetClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.SClass || msg.Data != "c" {
		t.Fatalf("unexpected set class msg %#v", msg)
	}
	jw.RemoveClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.RClass || msg.Data != "c" {
		t.Fatalf("unexpected remove class msg %#v", msg)
	}
	jw.SetValue("t", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected set value msg %#v", msg)
	}
	jw.Insert("t", "0", "<i>a</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Insert || msg.Data != "0\n<i>a</i>" {
		t.Fatalf("unexpected insert msg %#v", msg)
	}
	jw.Replace("t", "<i>b</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Replace || msg.Data != "<i>b</i>" {
		t.Fatalf("unexpected replace msg %#v", msg)
	}
	jw.Delete("t")
	if msg := nextBroadcast(t, jw); msg.What != what.Delete {
		t.Fatalf("unexpected delete msg %#v", msg)
	}
	jw.Append("t", "<em>c</em>")
	if msg := nextBroadcast(t, jw); msg.What != what.Append || msg.Data != "<em>c</em>" {
		t.Fatalf("unexpected append msg %#v", msg)
	}
	jw.JsCall("t", "fn", `{"a":1}`)
	if msg := nextBroadcast(t, jw); msg.What != what.Call || msg.Data != `fn={"a":1}` {
		t.Fatalf("unexpected jscall msg %#v", msg)
	}
}

func TestJaws_Redirect_unsafeRefused(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger
	// A script-bearing scheme must be refused and logged, never broadcast.
	jw.Redirect("javascript:alert(1)")
	if logger.err == nil || !strings.Contains(logger.err.Error(), "refusing unsafe redirect") {
		t.Fatalf("expected unsafe redirect to be logged and skipped, got %v", logger.err)
	}
}

func TestJaws_DirtyIllegalTagLogsNotPanics(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger
	// An illegal tag (a bare int) must be logged and skipped, like Request.Dirty
	// and the broadcast helpers, rather than panicking in production.
	jw.Dirty(42)
	if logger.err == nil || !errors.Is(logger.err, tag.ErrIllegalTagType) {
		t.Fatalf("expected illegal tag to be logged, got %v", logger.err)
	}
}

func TestBroadcast_ExpandsTagDestBeforeQueue(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tagger := testBroadcastTagGetter{}

	jw.Broadcast(wire.Message{
		Dest: tagger,
		What: what.Inner,
		Data: "x",
	})
	msg := nextBroadcast(t, jw)
	if msg.What != what.Inner || msg.Data != "x" {
		t.Fatalf("unexpected msg %#v", msg)
	}
	if got, ok := msg.Dest.(tag.Tag); !ok || got != tag.Tag("expanded") {
		t.Fatalf("expected expanded Tag destination, got %T(%#v)", msg.Dest, msg.Dest)
	}

	jw.Broadcast(wire.Message{
		Dest: []any{tagger, tag.Tag("extra")},
		What: what.Value,
		Data: "v",
	})
	msg = nextBroadcast(t, jw)
	if msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected msg %#v", msg)
	}
	dest, ok := msg.Dest.([]any)
	if !ok {
		t.Fatalf("expected []any destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
	if len(dest) != 2 || dest[0] != tag.Tag("expanded") || dest[1] != tag.Tag("extra") {
		t.Fatalf("unexpected expanded destination %#v", dest)
	}

	jw.Broadcast(wire.Message{
		Dest: "html-id",
		What: what.Delete,
	})
	msg = nextBroadcast(t, jw)
	if got, ok := msg.Dest.(string); !ok || got != "html-id" {
		t.Fatalf("expected raw html-id destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
}

func TestBroadcast_NoneDestination(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.Broadcast(wire.Message{
		Dest: []any{},
		What: what.Update,
		Data: "x",
	})

	select {
	case msg := <-jw.bcastCh:
		t.Fatalf("expected no pending broadcast, got %T(%#v)", msg.Dest, msg.Dest)
	default:
	}
}

func TestBroadcast_RejectsUnhashableTagDest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	// A statically comparable struct holding a func in an interface field panics
	// when used as a map key. Such a Dest must be rejected before it reaches the
	// Serve loop, where wantMessage's tagMap lookup would panic and tear the loop
	// down. Broadcast must not panic and must not queue the message in any build.
	type ifaceHolder struct{ any }
	var panicked bool
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		jw.Broadcast(wire.Message{What: what.Reload, Dest: ifaceHolder{any: func() {}}})
	}()

	if panicked {
		t.Error("Broadcast must not let an unhashable tag panic the caller")
	}
	select {
	case msg := <-jw.bcastCh:
		t.Fatalf("unhashable Dest was queued for the Serve loop: %T(%#v)", msg.Dest, msg.Dest)
	default:
	}
	if !errors.Is(logger.err, tag.ErrNotUsableAsTag) {
		t.Errorf("logged error = %v, want ErrNotUsableAsTag", logger.err)
	}
	if !errors.Is(logger.err, tag.ErrNotComparable) {
		t.Errorf("logged error = %v, want ErrNotComparable", logger.err)
	}
}

func TestBroadcast_ReturnsWhenClosedAndQueueFull(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.Broadcast(wire.Message{What: what.Alert, Data: "info\nfirst"})
	jw.Close()

	done := make(chan struct{})
	go func() {
		jw.Broadcast(wire.Message{What: what.Alert, Data: "info\nsecond"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked after close")
	}

	msg := nextBroadcast(t, jw)
	if msg.Data != "info\nfirst" {
		t.Fatalf("unexpected queued message %#v", msg)
	}
	select {
	case extra := <-jw.bcastCh:
		t.Fatalf("unexpected extra message after close %#v", extra)
	default:
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestJaws_GenerateHeadHTML_StoresCSPBuiltBySecureHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	extras := []string{
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
		"https://images.example.com/logo.png",
	}
	if err = jw.GenerateHeadHTML(extras...); err != nil {
		t.Fatal(err)
	}

	urls := []*url.URL{
		mustParseURL(t, jw.serveCSS.Name),
		mustParseURL(t, jw.serveJS.Name),
	}
	for _, extra := range extras {
		urls = append(urls, mustParseURL(t, extra))
	}

	wantCSP := secureheaders.BuildContentSecurityPolicy(urls)
	if got := jw.ContentSecurityPolicy(); got != wantCSP {
		t.Fatalf("unexpected CSP:\nwant: %q\ngot:  %q", wantCSP, got)
	}
}

// TestJaws_GenerateHeadHTML_DeduplicatesBuiltinResources verifies that re-listing the
// built-in JaWS JS and CSS resources as extras is a no-op: both are already prepended,
// so the regenerated head HTML and CSP must be identical to the built-ins-only output
// produced by New. This guards the symmetric dedup of both built-in paths.
func TestJaws_GenerateHeadHTML_DeduplicatesBuiltinResources(t *testing.T) {
	jw, err := New() // New calls GenerateHeadHTML() with no extras: built-ins only.
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	baseHead := jw.headPrefix
	baseCSP := jw.ContentSecurityPolicy()

	if err = jw.GenerateHeadHTML(jw.serveJS.Name, jw.serveCSS.Name); err != nil {
		t.Fatal(err)
	}
	if got := jw.headPrefix; got != baseHead {
		t.Fatalf("re-listing built-ins changed head HTML:\nbase: %q\ngot:  %q", baseHead, got)
	}
	if got := jw.ContentSecurityPolicy(); got != baseCSP {
		t.Fatalf("re-listing built-ins changed CSP:\nbase: %q\ngot:  %q", baseCSP, got)
	}
}

func TestJaws_GenerateHeadHTML_PropagatesResourceParseErrors(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	err = jw.GenerateHeadHTML("https://bad host")
	if err == nil {
		t.Fatal("expected parse error for extra resource URL")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestJaws_SecureHeadersMiddleware_UsesJawsCSP(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err = jw.GenerateHeadHTML(
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
	); err != nil {
		t.Fatal(err)
	}
	wantCSP := jw.ContentSecurityPolicy()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	rr := httptest.NewRecorder()
	jw.SecureHeadersMiddleware(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatal("expected wrapped handler to be called")
	}
	if got := rr.Result().StatusCode; got != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, got)
	}

	hdr := rr.Result().Header
	if got := hdr.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected CSP %q, got %q", wantCSP, got)
	}
	defaultHeaders := secureheaders.DefaultHeaders()
	if got := hdr.Get("Strict-Transport-Security"); got != defaultHeaders.Get("Strict-Transport-Security") {
		t.Fatalf("expected HSTS %q, got %q", defaultHeaders.Get("Strict-Transport-Security"), got)
	}
}

func TestJaws_SecureHeadersMiddleware_UsesUpdatedJawsCSP(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	oldCSP := jw.ContentSecurityPolicy()
	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	if err = jw.GenerateHeadHTML("https://cdn.example.test/app.js"); err != nil {
		t.Fatal(err)
	}
	wantCSP := jw.ContentSecurityPolicy()
	if wantCSP == oldCSP {
		t.Fatal("expected GenerateHeadHTML to change the CSP")
	}

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://example.test/", nil))

	if got := rr.Result().Header.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected updated CSP %q, got %q", wantCSP, got)
	}
}

func TestJaws_SecureHeadersMiddleware_DoesNotModifyDefaultHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	wantCSP := jw.ContentSecurityPolicy()
	defaultCSP := secureheaders.DefaultHeaders().Get("Content-Security-Policy")
	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	hdr := rr.Result().Header

	if got := hdr.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected CSP %q, got %q", wantCSP, got)
	}
	if got := secureheaders.DefaultHeaders().Get("Content-Security-Policy"); got != defaultCSP {
		t.Fatalf("expected default CSP %q, got %q", defaultCSP, got)
	}
}

func TestJaws_SecureHeadersMiddleware_DoesNotTrustForwardedHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if got := rr.Result().Header.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS over HTTP request with forwarded proto, got %q", got)
	}
}

func TestJaws_distributeDirt_AscendingOrder(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := &Request{}
	jw.mu.Lock()
	jw.requests[1] = rq
	jw.dirty[tag.Tag("fourth")] = 4
	jw.dirty[tag.Tag("second")] = 2
	jw.dirty[tag.Tag("fifth")] = 5
	jw.dirty[tag.Tag("first")] = 1
	jw.dirty[tag.Tag("third")] = 3
	jw.dirtOrder = 5
	jw.mu.Unlock()

	if got, want := jw.distributeDirt(), 5; got != want {
		t.Fatalf("distributeDirt() = %d, want %d", got, want)
	}

	rq.mu.RLock()
	got := append([]any(nil), rq.todoDirt...)
	rq.mu.RUnlock()

	want := []any{
		tag.Tag("first"),
		tag.Tag("second"),
		tag.Tag("third"),
		tag.Tag("fourth"),
		tag.Tag("fifth"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dirty tags = %#v, want %#v", got, want)
	}
}

func TestJaws_GenerateHeadHTMLConcurrentWithHeadHTML(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if err := jw.GenerateHeadHTML("/a.js", "/b.css"); err != nil {
					t.Error(err)
					return
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
				var buf bytes.Buffer
				if err := rq.HeadHTML(&buf); err != nil {
					t.Error(err)
				}
				jw.recycle(rq)
			}
		}
	}()

	// Let the two goroutines hammer the shared state concurrently for a fixed
	// window so the race detector can observe genuine parallel access; a synctest
	// bubble would serialize them and defeat the purpose.
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestCoverage_IDAndLookupHelpers(t *testing.T) {
	if a, b := NextID(), NextID(); b <= a {
		t.Fatalf("expected increasing ids, got %d then %d", a, b)
	}
	if got := string(AppendID([]byte("x"))); !strings.HasPrefix(got, "x") || len(got) <= 1 {
		t.Fatalf("unexpected append id result %q", got)
	}
	if got := MakeID(); !strings.HasPrefix(got, "jaws.") {
		t.Fatalf("unexpected id %q", got)
	}

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tmpl := template.Must(template.New("it").Parse(`ok`))
	_ = jw.AddTemplateLookuper(tmpl)
	if got := jw.LookupTemplate("it"); got == nil {
		t.Fatal("expected found template")
	}
	if got := jw.LookupTemplate("missing"); got != nil {
		t.Fatal("expected missing template")
	}
	_ = jw.RemoveTemplateLookuper(nil)
	_ = jw.RemoveTemplateLookuper(tmpl)

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("expected request")
	}
	if got := jw.RequestCount(); got != 1 {
		t.Fatalf("expected one request, got %d", got)
	}
	jw.recycle(rq)
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("expected zero requests, got %d", got)
	}
}

func TestCoverage_CookieParseAndIP(t *testing.T) {
	h := http.Header{}
	h.Add("Cookie", `a=1; jaws=`+key.Key(11).String()+`; x=2`)
	h.Add("Cookie", `jaws="`+key.Key(12).String()+`"`)
	h.Add("Cookie", `jaws=not-a-key`)

	ids := getCookieSessionsIDs(h, "jaws")
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 12 {
		t.Fatalf("unexpected cookie ids %#v", ids)
	}

	if got := parseIP("127.0.0.1:1234"); !got.IsValid() {
		t.Fatalf("expected parsed host:port ip, got %v", got)
	}
	if got := parseIP("::1"); !got.IsValid() {
		t.Fatalf("expected parsed direct ip, got %v", got)
	}
	if got := parseIP(""); got.IsValid() {
		t.Fatalf("expected invalid ip for empty remote addr, got %v", got)
	}
}

func TestJaws_clientIP(t *testing.T) {
	mustIP := func(s string) netip.Addr {
		t.Helper()
		ip, err := netip.ParseAddr(s)
		if err != nil {
			t.Fatalf("bad test ip %q: %v", s, err)
		}
		return ip
	}

	// A nil request yields an invalid address.
	if got := (&Jaws{}).clientIP(nil); got.IsValid() {
		t.Errorf("clientIP(nil) = %v, want invalid", got)
	}

	newReq := func(remoteAddr string, hdrs map[string]string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remoteAddr
		for k, v := range hdrs {
			r.Header.Set(k, v)
		}
		return r
	}

	tests := []struct {
		name  string
		trust bool
		addr  string
		hdrs  map[string]string
		want  netip.Addr
	}{
		{
			"untrusted ignores forwarded headers", false, "203.0.113.9:443",
			map[string]string{"X-Forwarded-For": "198.51.100.7"},
			mustIP("203.0.113.9"),
		},
		{
			"trusted uses leftmost X-Forwarded-For", true, "127.0.0.1:1234",
			map[string]string{"X-Forwarded-For": "198.51.100.7, 70.41.3.18, 127.0.0.1"},
			mustIP("198.51.100.7"),
		},
		{
			"trusted falls back to X-Real-IP", true, "127.0.0.1:1234",
			map[string]string{"X-Real-Ip": "198.51.100.23"},
			mustIP("198.51.100.23"),
		},
		{
			"trusted falls back to RemoteAddr when headers invalid", true, "203.0.113.9:443",
			map[string]string{"X-Forwarded-For": "not-an-ip", "X-Real-Ip": "garbage"},
			mustIP("203.0.113.9"),
		},
		{"trusted with no forwarded headers uses RemoteAddr", true, "203.0.113.9:443", nil, mustIP("203.0.113.9")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw := &Jaws{TrustForwardedHeaders: tt.trust}
			if got := jw.clientIP(newReq(tt.addr, tt.hdrs)); got.Compare(tt.want) != 0 {
				t.Errorf("clientIP = %v, want %v", got, tt.want)
			}
		})
	}

	// forwardedClientIP reports ok=false when neither header carries a valid IP.
	if _, ok := forwardedClientIP(http.Header{}); ok {
		t.Error("forwardedClientIP with no headers should report ok=false")
	}
}

func TestCoverage_NonZeroRandomAndPanic(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	// First random value is zero, second is one.
	zeroThenOne := append(make([]byte, 8), []byte{1, 0, 0, 0, 0, 0, 0, 0}...)
	jw.kg = bufio.NewReader(bytes.NewReader(zeroThenOne))
	if got := jw.nonZeroRandomLocked(); got != 1 {
		t.Fatalf("unexpected non-zero random value %d", got)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on random source read error")
		}
	}()
	jw.kg = bufio.NewReader(errReader{})
	_ = jw.nonZeroRandomLocked()
}

func TestJaws_ServeWithTimeoutBounds(t *testing.T) {
	// Min interval clamp path.
	jwMin, err := New()
	if err != nil {
		t.Fatal(err)
	}
	doneMin := make(chan struct{})
	go func() {
		jwMin.ServeWithTimeout(time.Nanosecond)
		close(doneMin)
	}()
	jwMin.Close()
	select {
	case <-doneMin:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout(min)")
	}

	// Max interval clamp path.
	jwMax, err := New()
	if err != nil {
		t.Fatal(err)
	}
	doneMax := make(chan struct{})
	go func() {
		jwMax.ServeWithTimeout(10 * time.Second)
		close(doneMax)
	}()
	jwMax.Close()
	select {
	case <-doneMax:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout(max)")
	}
}

func waitForServeLoop(t *testing.T, jw *Jaws) {
	t.Helper()
	select {
	case jw.subCh <- subscription{}:
	case <-jw.Done():
		t.Fatal("jaws closed before serve loop was ready")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for serve loop to receive subscription rendezvous")
	}
}

func TestJaws_ServeWithTimeoutRejectsDuplicateLoop(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const firstTimeout = time.Hour
	firstDone := make(chan struct{})
	go func() {
		jw.ServeWithTimeout(firstTimeout)
		close(firstDone)
	}()
	waitForServeLoop(t, jw)

	logger := &captureErrorLogger{}
	jw.Logger = logger
	secondDone := make(chan any, 1)
	go func() {
		defer func() {
			secondDone <- recover()
		}()
		jw.ServeWithTimeout(time.Nanosecond)
	}()

	select {
	case recovered := <-secondDone:
		if deadlock.Debug {
			err, ok := recovered.(error)
			if !ok || !errors.Is(err, ErrServeAlreadyRunning) {
				t.Fatalf("duplicate ServeWithTimeout panic = %v, want ErrServeAlreadyRunning", recovered)
			}
		} else if recovered != nil {
			t.Fatalf("duplicate ServeWithTimeout panicked in production mode: %v", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("duplicate ServeWithTimeout did not return promptly")
	}

	if !errors.Is(logger.err, ErrServeAlreadyRunning) {
		t.Fatalf("logged error = %v, want ErrServeAlreadyRunning", logger.err)
	}
	if got := jw.getWebSocketTimeout(); got != firstTimeout {
		t.Fatalf("webSocketTimeout = %v, want %v", got, firstTimeout)
	}

	jw.Close()
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first ServeWithTimeout did not stop after Close")
	}
}

func TestJaws_ServeWithTimeoutFullSubscriberChannel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
		msgCh := make(chan wire.Message) // unbuffered: always full when nobody receives
		done := make(chan struct{})
		go func() {
			jw.ServeWithTimeout(50 * time.Millisecond)
			close(done)
		}()
		jw.subCh <- subscription{msgCh: msgCh, rq: rq}
		waitForServeLoop(t, jw)
		jw.bcastCh <- wire.Message{What: what.Alert, Data: "x"}

		// Once the Serve loop is durably blocked again it has processed the
		// broadcast, found msgCh full, and killed (closed) the subscription.
		synctest.Wait()
		select {
		case _, ok := <-msgCh:
			if ok {
				t.Fatal("expected subscriber channel to be closed when full")
			}
		default:
			t.Fatal("expected subscriber channel to be closed when full")
		}

		jw.Close()
		<-done
	})
}

var headerContentGZip = []string{"gzip"}

type errResponseWriter struct {
	code      int
	header    http.Header
	writeErr  error
	writeCall int
}

func (w *errResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *errResponseWriter) Write(p []byte) (int, error) {
	w.writeCall++
	return 0, w.writeErr
}

func TestServeHTTP_GetJavascript(t *testing.T) {
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	is := newTestHelper(t)

	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)

	req := httptest.NewRequest("", jw.serveJS.Name, nil)
	req.Header.Add("Accept-Encoding", "blepp")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(assets.JavascriptText))
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".js")})
	is.Equal(w.Header()["Content-Encoding"], nil)

	req = httptest.NewRequest("", jw.serveJS.Name, nil)
	req.Header.Add("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".js")})
	is.Equal(w.Header()["Content-Encoding"], headerContentGZip)

	gr, err := gzip.NewReader(w.Body)
	is.NoErr(err)
	b, err := io.ReadAll(gr)
	is.NoErr(err)
	is.NoErr(gr.Close())
	is.Equal(len(assets.JavascriptText), len(b))
	is.Equal(string(assets.JavascriptText), string(b))
}

func TestServeHTTP_GetCSS(t *testing.T) {
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	is := newTestHelper(t)

	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)

	req := httptest.NewRequest("", jw.serveCSS.Name, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(assets.JawsCSS))
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".css")})
}

func TestServeHTTP_GetPing(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", "/jaws/.ping", nil)
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
	is.Equal(len(w.Body.Bytes()), 0)
	is.Equal(w.Header()["Content-Length"], nil)
	is.Equal(w.Code, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodPost, "/jaws/.ping", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusMethodNotAllowed)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/jaws/.pong", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/something_else", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	jw.Close()

	req = httptest.NewRequest("", "/jaws/.ping", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusServiceUnavailable)
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
}

func TestServeHTTP_GetKey(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", "/jaws/", nil)
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/jaws/12345678", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	w = httptest.NewRecorder()
	rq := jw.NewRequest(req)
	req = httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusUpgradeRequired)
	is.Equal(w.Header()["Cache-Control"], nil)
}

func TestServeHTTP_Noscript(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	w := httptest.NewRecorder()
	rq := jw.NewRequest(httptest.NewRequest("", "/", nil))
	req := httptest.NewRequest("", "/jaws/"+rq.JawsKeyString()+"/noscript", nil)
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNoContent)
}

func TestServeHTTP_UnknownKeySuffixDoesNotClaimRequest(t *testing.T) {
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	for _, tt := range []struct {
		name string
		path func(string) string
	}{
		{
			name: "trailing slash",
			path: func(k string) string { return "/jaws/" + k + "/" },
		},
		{
			name: "unknown suffix",
			path: func(k string) string { return "/jaws/" + k + "/unknown" },
		},
		{
			name: "unknown suffix before noscript",
			path: func(k string) string { return "/jaws/" + k + "/unknown/noscript" },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			is := newTestHelper(t)
			hr := httptest.NewRequest(http.MethodGet, "/", nil)
			rq := jw.NewRequest(hr)
			key := rq.JawsKeyString()

			req := httptest.NewRequest(http.MethodGet, tt.path(key), nil)
			req.RemoteAddr = hr.RemoteAddr
			w := httptest.NewRecorder()
			jw.ServeHTTP(w, req)
			is.Equal(w.Code, http.StatusNotFound)

			req = httptest.NewRequest(http.MethodGet, "/jaws/"+key, nil)
			req.RemoteAddr = hr.RemoteAddr
			w = httptest.NewRecorder()
			jw.ServeHTTP(w, req)
			is.Equal(w.Code, http.StatusUpgradeRequired)
		})
	}
}

func TestServeHTTP_TailScript_UnknownSuffixDoesNotDrain(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	rq.NewElement(&testUi{}).SetClass("cls")

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString()+"/unknown", nil)
	req.RemoteAddr = hr.RemoteAddr
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)

	req = httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(strings.Contains(w.Body.String(), `classList?.add("cls");`), true)
}

func TestServeHTTP_TailScript(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	item := &testUi{}
	e := rq.NewElement(item)
	e.SetAttr("title", `</script><img onerror=alert(1) src=x>`)
	e.SetClass("cls")
	e.SetInner("kept")

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Header()["Content-Type"], headerContentTypeJavaScript)
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
	is.Equal(strings.Contains(w.Body.String(), `setAttribute("title","\x3c/script>\x3cimg onerror=alert(1) src=x>");`), true)
	is.Equal(strings.Contains(w.Body.String(), `classList?.add("cls");`), true)
	is.Equal(strings.Contains(w.Body.String(), "kept"), false)
	is.Equal(jw.RequestCount(), 1)
}

func TestServeHTTP_TailScript_EndpointIsPerRequest(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)

	req = httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNoContent)
}

// TestServeHTTP_TailScript_RejectsRecycledKey covers the recycled-request behavior
// of the /jaws/.tail endpoint: recycling deletes the key from jw.requests, so a tail
// fetch for the old key misses the map and returns 404, while the reused pooled
// object drains only its own freshly queued content (its queue was reset by
// clearLocked), never the recycled request's. This exercises the map-deletion 404
// and content isolation; the concurrent drain-under-lock guarantee (holding jw.mu
// across drainTailScript) is exercised separately, under the race detector, by
// TestRequest_TailScriptConcurrentWithRecycle.
func TestServeHTTP_TailScript_RejectsRecycledKey(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	stale := jw.NewRequest(hr)
	stale.NewElement(&testUi{}).SetClass("stale")
	staleKey := stale.JawsKeyString()

	// Recycle the request and create a new one under a fresh key. The pool typically
	// hands back the same struct, so the content check below also guards that a
	// reused object carries none of the recycled request's queued content; it holds
	// whether or not the pool reused the struct.
	jw.recycle(stale)
	rq := jw.NewRequest(hr)
	rq.NewElement(&testUi{}).SetClass("fresh")

	// Old key was deleted from jw.requests on recycle, so the lookup misses and
	// nothing is drained.
	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+staleKey, nil)
	req.RemoteAddr = hr.RemoteAddr
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)

	// The reused request's own tail fetch still returns its own content.
	req = httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(strings.Contains(w.Body.String(), `classList?.add("fresh");`), true)
	is.Equal(strings.Contains(w.Body.String(), "stale"), false)
}

// TestServeHTTP_TailScript_IPMismatch verifies that a /jaws/.tail fetch from a
// different client IP than the initial request is rejected (404) and does not drain
// the one-shot tail, so the legitimate client can still fetch its own content. This
// mirrors the IP binding the WebSocket claim path enforces.
func TestServeHTTP_TailScript_IPMismatch(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	hr.RemoteAddr = "203.0.113.1:1111"
	rq := jw.NewRequest(hr)
	rq.NewElement(&testUi{}).SetClass("cls")

	// A fetch from a different (non-loopback) IP is rejected and must not consume the
	// one-shot tail.
	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = "203.0.113.2:2222"
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)

	// The legitimate client (same IP, any port) still gets its content: the mismatched
	// fetch above did not set tailsent.
	req = httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = "203.0.113.1:3333"
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(strings.Contains(w.Body.String(), `classList?.add("cls");`), true)
}

func TestServeHTTP_TailScript_WriteError(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	item := &testUi{}
	rq.NewElement(item).SetClass("cls")

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w := &errResponseWriter{writeErr: errors.New("write failed")}
	jw.ServeHTTP(w, req)

	is.Equal(w.writeCall > 0, true)
	is.Equal(w.Header()["Content-Type"], headerContentTypeJavaScript)
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
	is.Equal(jw.RequestCount(), 1)
	is.Equal(rq.Context().Err() != nil, true)
}

func TestJaws_cancelIfCurrent_IgnoresStaleRequest(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	stale := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	jawsKey := stale.JawsKey

	// The /jaws/.tail handler snapshots the Request before writing the response;
	// recycle the snapshot and create a new request so the pool can hand the
	// stale pointer to a different connection, as can happen before a write
	// error triggers a cancel.
	jw.recycle(stale)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	jw.cancelIfCurrent(jawsKey, stale, errors.New("write failed"))

	// Whether or not the pool reused the object for rq (it normally does here,
	// which is exactly the stale-cancel scenario), rq must stay live.
	is.NoErr(rq.Context().Err())
}

func TestJaws_Session(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	dot := tag.Tag("123")

	h := rq.Jaws.Session(rq.Jaws.Handler("div", "testtemplate", dot))
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)

	if sess := rq.Jaws.GetSession(r); sess != nil {
		t.Error("session already exists")
	}

	h.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1">123</div>` {
		t.Error(got)
	}

	sess := rq.Jaws.GetSession(r)
	if sess == nil {
		t.Error("no session")
	}
}
