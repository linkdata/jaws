package jaws

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/deadlock"
)

func TestReqStateString(t *testing.T) {
	for s, want := range map[reqState]string{
		reqUnclaimable: "unclaimable",
		reqPending:     "pending",
		reqClaimed:     "claimed",
		reqRunning:     "running",
		reqFinished:    "finished",
		reqState(99):   "reqState(99)",
	} {
		if got := s.String(); got != want {
			t.Errorf("reqState(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}

// TestRequestStateTransitions covers the lifecycle state machine: the normal
// pending -> claimed -> running -> finished path, the direct finish paths a
// never-claimed or claimed-but-not-running Request takes, and that duplicate
// claim/startServe are rejected (a normal CAS failure) without moving the state.
func TestRequestStateTransitions(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	waitForServeLoop(t, jw)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	t.Run("pending->claimed->running->finished", func(t *testing.T) {
		rq := jw.NewRequest(r)
		if got := rq.loadState(); got != reqPending {
			t.Fatalf("after NewRequest = %v, want pending", got)
		}
		if jw.UseRequest(rq.JawsKey, r) != rq {
			t.Fatal("claim failed")
		}
		if got := rq.loadState(); got != reqClaimed {
			t.Fatalf("after claim = %v, want claimed", got)
		}
		if !rq.startServe() {
			t.Fatal("startServe failed")
		}
		if got := rq.loadState(); got != reqRunning {
			t.Fatalf("after startServe = %v, want running", got)
		}
		jw.recycle(rq)
		if got := rq.loadState(); got != reqFinished {
			t.Fatalf("after recycle = %v, want finished", got)
		}
	})

	t.Run("recycle_never_claimed_pending->finished", func(t *testing.T) {
		rq := jw.NewRequest(r)
		if got := rq.loadState(); got != reqPending {
			t.Fatalf("state = %v, want pending", got)
		}
		jw.recycle(rq)
		if got := rq.loadState(); got != reqFinished {
			t.Fatalf("after recycle = %v, want finished", got)
		}
	})

	t.Run("retire_claimed_not_running->finished", func(t *testing.T) {
		rq := jw.NewRequest(r)
		if jw.UseRequest(rq.JawsKey, r) != rq {
			t.Fatal("claim failed")
		}
		if got := rq.loadState(); got != reqClaimed {
			t.Fatalf("state = %v, want claimed", got)
		}
		jw.mu.Lock()
		jw.retireNonRunningRequestLocked(rq)
		jw.mu.Unlock()
		if got := rq.loadState(); got != reqFinished {
			t.Fatalf("after retire = %v, want finished", got)
		}
	})

	t.Run("double_claim_and_double_startServe_rejected", func(t *testing.T) {
		rq := jw.NewRequest(r)
		if jw.UseRequest(rq.JawsKey, r) != rq {
			t.Fatal("first claim failed")
		}
		if jw.UseRequest(rq.JawsKey, r) != nil {
			t.Fatal("second claim should fail")
		}
		if got := rq.loadState(); got != reqClaimed {
			t.Fatalf("after double claim = %v, want claimed", got)
		}
		if !rq.startServe() {
			t.Fatal("first startServe failed")
		}
		if rq.startServe() {
			t.Fatal("second startServe should fail")
		}
		if got := rq.loadState(); got != reqRunning {
			t.Fatalf("after double startServe = %v, want running", got)
		}
		jw.recycle(rq)
	})

	t.Run("terminal_stays_finished", func(t *testing.T) {
		rq := jw.NewRequest(r)
		jw.recycle(rq)
		if got := rq.loadState(); got != reqFinished {
			t.Fatalf("state = %v, want finished", got)
		}
		// A finished Request is not claimable, not startable, and a second recycle is
		// a no-op (the map-identity guard fails), so it never resurrects.
		if jw.UseRequest(rq.JawsKey, r) != nil {
			t.Fatal("finished Request must not be claimable")
		}
		if rq.startServe() {
			t.Fatal("finished Request must not start serving")
		}
		jw.recycle(rq)
		if got := rq.loadState(); got != reqFinished {
			t.Fatalf("after double recycle = %v, want finished", got)
		}
	})
}

// TestFinishLockedTerminalStatePanicsInDebug verifies the debug-only invariant
// assertion in finishLocked: calling it on a terminal (non-live) state panics in
// debug/race builds, catching a double-finish or terminal-state resurrection. The
// assertion is compiled out in release builds, so this only exercises the panic
// when deadlock.Debug is set (the module's tests always run with -race).
func TestFinishLockedTerminalStatePanicsInDebug(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("finishLocked assertion is only active when deadlock.Debug is set (run with -race)")
	}
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := &Request{Jaws: jw}
	rq.storeState(reqFinished)

	defer func() {
		if recover() == nil {
			t.Error("finishLocked on a terminal state did not panic")
		}
	}()
	// finishLocked requires both jw.mu and rq.mu (jw.mu is acquired first, matching the
	// production lock order in recycleLockedWithCause/retireNonRunningRequestCoreLocked).
	jw.mu.Lock()
	defer jw.mu.Unlock()
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.finishLocked()
}

// TestClaimPostCloseReturnsCauseNotAlreadyClaimed covers the claim fast path for a
// terminal (reqUnclaimable) Request: it must return the cancellation cause, as an
// unclaimed canceled Request always has, not ErrRequestAlreadyClaimed.
func TestClaimPostCloseReturnsCauseNotAlreadyClaimed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Close()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(r)
	if got := rq.loadState(); got != reqUnclaimable {
		t.Fatalf("post-Close state = %v, want unclaimable", got)
	}
	err = rq.claim(r)
	if err == nil {
		t.Fatal("post-Close claim should fail")
	}
	if errors.Is(err, ErrRequestAlreadyClaimed) {
		t.Fatalf("post-Close claim = %v, want the cancellation cause, not ErrRequestAlreadyClaimed", err)
	}
	if rq.loadState().claimed() {
		t.Fatal("post-Close Request must not be marked claimed")
	}
}

// TestServe_DuplicatePanicsWithoutDisturbingFirst verifies the checked transition:
// a second TestServe on an already-running Request panics (failed reqClaimed ->
// reqRunning) and must not recycle the Request or stop the first serve.
func TestServe_DuplicatePanicsWithoutDisturbingFirst(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	waitForServeLoop(t, jw)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(r)
	if jw.UseRequest(rq.JawsKey, r) != rq {
		t.Fatal("claim failed")
	}

	inCh, _, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	<-readyCh

	func() {
		defer func() {
			if recover() == nil {
				t.Error("duplicate TestServe did not panic")
			}
		}()
		jw.TestServe(rq, func(any) {})
	}()

	// The first serve must be undisturbed: still running, stoppable via its own inCh.
	select {
	case <-doneCh:
		t.Fatal("first TestServe stopped after the duplicate")
	default:
	}
	close(inCh)
	<-doneCh
}

// TestServe_CloseRaceIsSafe races Jaws.Close against TestServe. Exactly one outcome
// is legal: TestServe won and served cleanly, or Close won and TestServe refused via
// one of its "jaws: TestServe" setup-failure panics (the subscription timed out
// against the stopped Serve loop, or the checked reqClaimed->reqRunning transition
// failed). Any other panic — a terminal->running resurrection, a nil dereference, a
// double-finish assertion — is a bug. Whichever side wins, the Request must end
// reqFinished and must never be resurrected from a terminal state. Run with -race.
func TestServe_CloseRaceIsSafe(t *testing.T) {
	for i := range 50 {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		go jw.Serve()
		waitForServeLoop(t, jw)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		rq := jw.NewRequest(r)
		if jw.UseRequest(rq.JawsKey, r) != rq {
			t.Fatal("claim failed")
		}

		// Only the TestServe goroutine touches gotPanic and served; the WaitGroup's
		// Done->Wait edge publishes both to the assertions below without a data race.
		var (
			wg       sync.WaitGroup
			gotPanic any
			served   bool
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			jw.Close()
		}()
		go func() {
			defer wg.Done()
			defer func() { gotPanic = recover() }()
			inCh, _, _, readyCh, doneCh := jw.TestServe(rq, func(any) {})
			<-readyCh
			close(inCh)
			<-doneCh
			served = true
		}()
		wg.Wait()

		if gotPanic != nil {
			if msg, ok := gotPanic.(string); !ok || !strings.HasPrefix(msg, "jaws: TestServe") {
				t.Fatalf("iteration %d: unexpected panic %#v", i, gotPanic)
			}
			if served {
				t.Fatalf("iteration %d: TestServe both served and panicked", i)
			}
		}
		if got := rq.loadState(); got != reqFinished {
			t.Fatalf("iteration %d: final state = %v, want finished", i, got)
		}
	}
}
