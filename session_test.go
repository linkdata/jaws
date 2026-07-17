package jaws

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestSession_Object(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	sessionId := key.Key(0x12345)
	var sess *Session
	// Set/Get on nil Session is ignored
	sess.Set("foo", "bar")
	if x := sess.Get("foo"); x != nil {
		t.Error(x)
	}

	sess = newSession(jw, sessionId, netip.Addr{}, false)

	if sess.Jaws() != jw {
		t.Fatal("Jaws pointer mismatch")
	}

	sess.Set("foo", "bar")
	if x := sess.Get("foo"); x != "bar" {
		t.Error(x)
	}

	sess.Set("foo", nil)
	if x := sess.Get("foo"); x != nil {
		t.Error(x)
	}

	cookie := sess.Cookie()

	if jw.CookieName != cookie.Name {
		t.Error(cookie.Name)
	}
	if sessionId.String() != cookie.Value {
		t.Error(cookie.Value)
	}
	if uint64(sessionId) != sess.ID() {
		t.Error(sess.ID())
	}
	if sess.IP().IsValid() {
		t.Error(sess.IP())
	}
	sess.Reload()
}

func TestSession_CookieSecureMatchesRequest(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	sHTTP := jw.NewSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	if sHTTP == nil {
		t.Fatal("expected session")
	}
	if sHTTP.Cookie() == nil || sHTTP.Cookie().Secure {
		t.Fatal("expected insecure cookie for http request")
	}

	sHTTPS := jw.NewSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "https://example.test/", nil))
	if sHTTPS == nil {
		t.Fatal("expected session")
	}
	if sHTTPS.Cookie() == nil || !sHTTPS.Cookie().Secure {
		t.Fatal("expected secure cookie for https request")
	}

	// By default forwarded headers are not trusted, so a forwarded-as-https
	// request over plain HTTP must still yield an insecure cookie.
	hrForwarded := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	hrForwarded.Header.Set("X-Forwarded-Proto", "https")
	sForwarded := jw.NewSession(httptest.NewRecorder(), hrForwarded)
	if sForwarded == nil {
		t.Fatal("expected session")
	}
	if sForwarded.Cookie() == nil || sForwarded.Cookie().Secure {
		t.Fatal("expected insecure cookie when forwarded headers are not trusted")
	}

	// With TrustForwardedHeaders enabled, the same request yields a secure cookie.
	jwTrust, _ := New()
	defer jwTrust.Close()
	jwTrust.TrustForwardedHeaders = true
	hrTrusted := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	hrTrusted.Header.Set("X-Forwarded-Proto", "https")
	sTrusted := jwTrust.NewSession(httptest.NewRecorder(), hrTrusted)
	if sTrusted == nil {
		t.Fatal("expected session")
	}
	if sTrusted.Cookie() == nil || !sTrusted.Cookie().Secure {
		t.Fatal("expected secure cookie when forwarded headers are trusted")
	}
}

func TestSession_Use(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	go jw.ServeWithTimeout(time.Second)
	var wantSess *Session
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3" {
			r.RemoteAddr = "10.4.5.6:78"
		}

		if strings.HasPrefix(r.URL.Path, "/jaws/") {
			jw.ServeHTTP(w, r)
			return
		}

		var sb strings.Builder
		sess := jw.GetSession(r)
		rq := jw.NewRequest(r).Writer(&sb)
		if sess != rq.Session() {
			t.Error(sess)
		}

		switch r.URL.Path {
		case "/":
			wantSess = jw.NewSession(w, r)
			wantSess.Set("foo", "bar")
		case "/2":
			if x := rq.Get("foo"); x != "bar" {
				t.Error(x)
			}
			rq.Set("foo", "baz")
		case "/3":
			if x := rq.Session(); x == wantSess {
				t.Error(x)
			}
			if x := rq.Get("foo"); x != nil {
				t.Error(x)
			}
		case "/4":
			if x := rq.Get("foo"); x != "baz" {
				t.Error(x)
			}
			rq.Set("foo", nil)
			rq.Set("bar", "quux")
		}
		w.WriteHeader(http.StatusOK)
		jw.UseRequest(rq.Request().JawsKey, r)
	})

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Error(len(cookies))
	}
	if cookies[0].Name != jw.CookieName {
		t.Error(cookies[0].Name)
	}
	if wantSess == nil {
		t.Error(wantSess)
	}
	if cookies[0].Value != wantSess.CookieValue() {
		t.Error(cookies[0].Value)
	}
	if wantSess == nil {
		t.Error(wantSess)
	}
	if x := wantSess.Get("foo"); x != "bar" {
		t.Error(x)
	}

	r2, err := http.NewRequest("GET", srv.URL+"/2", nil)
	if err != nil {
		t.Fatal(err)
	}
	r2.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	if x := wantSess.Get("foo"); x != "baz" {
		t.Error(x)
	}
	if resp == nil {
		t.Fatal("nil")
	}

	rp, err := http.NewRequest("GET", srv.URL+"/jaws/.ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	rp.AddCookie(cookies[0])
	resp, err = srv.Client().Do(rp)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil")
	}

	r3, err := http.NewRequest("GET", srv.URL+"/3", nil)
	if err != nil {
		t.Fatal(err)
	}
	r3.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r3)
	if err != nil {
		t.Fatal(err)
	}
	if x := wantSess.Get("foo"); x != "baz" {
		t.Error(x)
	}
	if resp == nil {
		t.Fatal("nil")
	}

	r4, err := http.NewRequest("GET", srv.URL+"/4", nil)
	if err != nil {
		t.Fatal(err)
	}
	r4.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r4)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("nil")
	}

	if x := wantSess.Get("foo"); x != nil {
		t.Error(x)
	}
	if x := wantSess.Get("bar"); x != "quux" {
		t.Error(x)
	}
	wantSess.Clear()
	if x := wantSess.Get("bar"); x != nil {
		t.Error(x)
	}
}

func TestSession_Requests(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	sessionId := key.Key(0x12345)
	sess := newSession(jw, sessionId, netip.Addr{}, false)
	if x := sess.Requests(); x != nil {
		t.Error(x)
	}
}

func TestSession_Broadcast(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := jw.NewSession(rr, hr)
	if sess == nil {
		t.Fatal("expected session")
	}

	rq1 := jw.NewRequest(hr)
	hr2 := httptest.NewRequest(http.MethodGet, "/2", nil)
	hr2.RemoteAddr = hr.RemoteAddr
	hr2.AddCookie(sess.Cookie())
	rq2 := jw.NewRequest(hr2)

	if got := rq1.Session(); got != sess {
		t.Fatalf("request 1 session mismatch: %v", got)
	}
	if got := rq2.Session(); got != sess {
		t.Fatalf("request 2 session mismatch: %v", got)
	}

	msg := wire.Message{What: what.Alert, Data: "info\nhello"}
	done := make(chan struct{})
	go func() {
		sess.Broadcast(msg)
		close(done)
	}()

	msg1 := nextBroadcast(t, jw)
	msg2 := nextBroadcast(t, jw)

	for i, got := range []wire.Message{msg1, msg2} {
		if got.What != msg.What || got.Data != msg.Data {
			t.Fatalf("message %d mismatch: %#v", i+1, got)
		}
		// Session.Broadcast targets each request by its key identity, not the
		// reusable *Request pointer.
		if _, ok := got.Dest.(key.Key); !ok {
			t.Fatalf("message %d destination type: %T", i+1, got.Dest)
		}
	}

	seen := map[key.Key]bool{}
	seen[msg1.Dest.(key.Key)] = true
	seen[msg2.Dest.(key.Key)] = true
	if !seen[rq1.JawsKey] || !seen[rq2.JawsKey] || len(seen) != 2 {
		t.Fatalf("expected broadcasts for both requests, got %#v", seen)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Session.Broadcast to finish")
	}

	jw.recycle(rq1)
	jw.recycle(rq2)
}

// TestSession_ProducersSkipDetached covers finished Request pointers retained by
// a Session snapshot. Both producers target only Requests that still belong to
// the Session.
func TestSession_ProducersSkipDetached(t *testing.T) {
	th := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := jw.NewSession(rr, hr)
	th.True(sess != nil)

	live := jw.NewRequest(hr)
	th.True(live.Session() == sess)

	// Session methods operate on a snapshot after releasing sess.mu. These entries
	// model finished pointers already detached from the Session; a finished Request
	// retains its original nonzero identity.
	detachedZero := &Request{Jaws: jw}
	detachedKey := &Request{Jaws: jw, JawsKey: key.Key(0x9876)}
	sess.mu.Lock()
	sess.requests = append(sess.requests, detachedZero, detachedKey)
	sess.mu.Unlock()

	// Session.Broadcast targets only the live request, never detached identities.
	done := make(chan struct{})
	go func() {
		sess.Broadcast(wire.Message{What: what.Alert, Data: "info\nhi"})
		close(done)
	}()
	got := nextBroadcast(t, jw)
	th.Equal(got.Dest, live.JawsKey)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Session.Broadcast")
	}
	select {
	case extra := <-jw.bcastCh:
		t.Fatalf("detached request must not broadcast, got %#v", extra)
	default:
	}

	// Session.Close reloads only the live request, never detached identities.
	closeDone := make(chan struct{})
	go func() {
		sess.Close()
		close(closeDone)
	}()
	got = nextBroadcast(t, jw)
	th.Equal(got.What, what.Reload)
	th.Equal(got.Dest, live.JawsKey)
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Session.Close")
	}
	select {
	case extra := <-jw.bcastCh:
		t.Fatalf("detached request must not broadcast, got %#v", extra)
	default:
	}

	jw.recycle(live)
}

// TestSessionCloseDoesNotReachLaterRequest covers a Request finishing after
// Session.Close snapshots it and before Close detaches it. A later Request must
// retain a distinct identity and must not receive the old Session's reload.
func TestSessionCloseDoesNotReachLaterRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	go jw.Serve()
	waitForServeLoop(t, jw)

	sessionHTTP := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	sess := jw.NewSession(httptest.NewRecorder(), sessionHTTP)
	first := jw.NewRequest(sessionHTTP)
	stale := jw.NewRequest(sessionHTTP)
	if first.Session() != sess || stale.Session() != sess {
		t.Fatal("test requests did not attach to the session")
	}

	// Close snapshots [first, stale], clears sess.requests, then blocks trying
	// to detach first. Observing the cleared Session proves stale is already in
	// Close's private snapshot before it finishes.
	firstLocked := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstReleased := make(chan struct{})
	go func() {
		first.mu.Lock()
		close(firstLocked)
		<-releaseFirst
		first.mu.Unlock()
		close(firstReleased)
	}()
	<-firstLocked
	releasedFirst := false
	defer func() {
		if !releasedFirst {
			close(releaseFirst)
			<-firstReleased
		}
	}()
	closeDone := make(chan struct{})
	go func() {
		_ = sess.Close()
		close(closeDone)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		sess.mu.RLock()
		snapshotted := sess.cookie.MaxAge < 0 && len(sess.requests) == 0
		sess.mu.RUnlock()
		if snapshotted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Session.Close did not reach its Request snapshot")
		}
		time.Sleep(time.Millisecond)
	}

	// Drive the stale Request through the real capability endpoint. The missing
	// WebSocket upgrade headers make ServeHTTP fail the upgrade and execute its
	// normal stopServe completion path after Session.Close has snapshotted it.
	staleEndpoint := httptest.NewRequest(http.MethodGet, "/jaws/"+stale.JawsKeyString(), nil)
	staleEndpoint.RemoteAddr = sessionHTTP.RemoteAddr
	jw.ServeHTTP(httptest.NewRecorder(), staleEndpoint)
	if stale.Context().Err() == nil {
		t.Fatal("failed WebSocket upgrade left stale Request live")
	}

	server := httptest.NewServer(jw)
	defer server.Close()
	unrelatedHTTP := httptest.NewRequest(http.MethodGet, server.URL+"/unrelated", nil)
	unrelatedHTTP.RemoteAddr = "127.0.0.1:2000"
	unrelated := jw.NewRequest(unrelatedHTTP)
	if unrelated == stale {
		t.Fatal("later client reused stale Request identity")
	}
	if unrelated.Session() != nil {
		t.Fatal("later Request retained old Session")
	}
	unrelatedKey := unrelated.JawsKey
	connected := make(chan struct{})
	unrelated.SetConnectFn(func(*Request) error {
		close(connected)
		return nil
	})
	hdr := http.Header{}
	hdr.Set("Origin", server.URL)
	conn, response, err := websocket.Dial(t.Context(), server.URL+"/jaws/"+unrelatedKey.String(), &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("WebSocket status = %d, want %d", response.StatusCode, http.StatusSwitchingProtocols)
	}
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("unrelated Request did not start its WebSocket")
	}

	close(releaseFirst)
	<-firstReleased
	releasedFirst = true
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Session.Close did not finish")
	}

	const marker = "post-close marker"
	jw.Broadcast(wire.Message{Dest: unrelatedKey, What: what.Alert, Data: marker})
	readCtx, cancelRead := context.WithTimeout(t.Context(), time.Second)
	defer cancelRead()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("reading marker from unrelated Request: %v", err)
	}
	if strings.Contains(string(data), what.Reload.String()+"\t") {
		t.Fatalf("old Session close reached unrelated later Request: %q", data)
	}
	if !strings.Contains(string(data), marker) {
		t.Fatalf("WebSocket message = %q, want marker %q", data, marker)
	}
}

func BenchmarkSessionBroadcast(b *testing.B) {
	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(jw.Close)
	sess := newSession(jw, key.Key(1), netip.Addr{}, false)
	rq := &Request{Jaws: jw, JawsKey: key.Key(2), session: sess}
	sess.requests = []*Request{rq}
	drainDone := make(chan struct{})
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for {
			select {
			case <-jw.bcastCh:
			case <-drainDone:
				return
			}
		}
	}()
	b.Cleanup(func() {
		close(drainDone)
		<-drained
	})
	msg := wire.Message{What: what.Reload}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sess.Broadcast(msg)
		}
	})
}

func TestSession_Delete(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer(t)
	defer ts.Close()

	// the test session is there
	sl := ts.jw.Sessions()
	if x := len(sl); x != 1 {
		t.Fatal(x)
	}
	if x := sl[0]; x != ts.sess {
		t.Fatal(x)
	}

	// session cookie seems ok
	cookie1 := &ts.sess.cookie
	if cookie1 != nil {
		if x := cookie1.Name; x != ts.jw.CookieName {
			t.Error(x)
		}
	} else {
		t.Fatal(cookie1)
	}

	// trying to get the session from another IP fails
	hr2 := httptest.NewRequest("GET", "/", nil)
	hr2.AddCookie(&ts.sess.cookie)
	hr2.RemoteAddr = "10.5.6.7:89"
	sess := ts.jw.GetSession(hr2)
	if x := sess; x != nil {
		t.Error(x)
	}

	// accessing from same IP but other port works
	host, port, _ := net.SplitHostPort(ts.hr.RemoteAddr)
	if port == "1" {
		port = "2"
	} else {
		port = "1"
	}
	hr2.RemoteAddr = net.JoinHostPort(host, port)
	sess = ts.jw.GetSession(hr2)
	if x := sess; x != ts.sess {
		t.Error(x)
	}

	rq2 := ts.jw.NewRequest(hr2)
	if x := rq2.Session(); x != ts.sess {
		t.Error(x)
	}

	// session should now have both requests listed
	rl := sess.Requests()
	if len(rl) != 2 {
		t.Error(len(rl))
	}
	if !slices.Contains(rl, ts.rq) {
		t.Errorf("%v missing from %v", ts.rq, rl)
	}
	if !slices.Contains(rl, rq2) {
		t.Errorf("%v missing from %v", rq2, rl)
	}

	byebyeItem := &testUi{}
	testRequestWriter{rq: ts.rq, Writer: httptest.NewRecorder()}.Register(byebyeItem, func(elem *Element, value string) error {
		sess2 := ts.jw.GetSession(elem.Request.Initial())
		if x := sess2; x != ts.sess {
			t.Error(x)
		}
		if x := sess2.cookie.MaxAge; x < 0 {
			t.Error(x)
		}

		cookie2 := sess2.Close()
		if x := cookie2; x == nil {
			t.Fatal(x)
		}
		if x := cookie2.MaxAge; x != -1 {
			t.Error(x)
		}
		if x := cookie2.Expires.IsZero(); !x {
			t.Error(x)
		}
		if x := cookie2.Name; x != cookie1.Name {
			t.Error(x)
		}
		if x := cookie2.Value; x != cookie1.Value {
			t.Error(x)
		}
		return nil
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if x := resp.StatusCode; x != http.StatusSwitchingProtocols {
		t.Error(x)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	msg := wire.WsMsg{Jid: jidForTag(ts.rq, byebyeItem), What: what.Input}
	ctx, cancel := context.WithCancel(ts.ctx)
	defer cancel()

	err = conn.Write(ctx, websocket.MessageText, msg.Append(nil))
	if err != nil {
		t.Fatal(err)
	}
	if x := ctx.Err(); x != nil {
		t.Fatal(x)
	}
	if x := ts.ctx.Err(); x != nil {
		t.Fatal(x)
	}

	type readResult struct {
		mt  websocket.MessageType
		b   []byte
		err error
	}

	resultChan := make(chan readResult)

	go func() {
		var rr readResult
		defer close(resultChan)
		rr.mt, rr.b, rr.err = conn.Read(ctx)
		resultChan <- rr
	}()

	if x := ts.ctx.Err(); x != nil {
		t.Fatal(x)
	}

	select {
	case <-th.C:
		th.Timeout()
	case rr, ok := <-resultChan:
		if ok {
			if x := rr.err; x != nil {
				t.Fatal(x)
			}
			if x := ctx.Err(); x != nil {
				t.Fatal(x)
			}
			if x := ts.ctx.Err(); x != nil {
				t.Fatal(x)
			}
			if x := sess.cookie.MaxAge; x != -1 {
				t.Error(x)
			}
			if x := rr.mt; x != websocket.MessageText {
				t.Error(x)
			}
			if x := string(rr.b); x != "Reload\t\t\"\"\n" {
				t.Error(x)
			}
		} else {
			t.Error("resultChan closed")
		}
	}
}

func TestSession_Cleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, _ := New()
		defer func() {
			jw.Close()
			synctest.Wait()
		}()

		rr := httptest.NewRecorder()
		hr := httptest.NewRequest("GET", "/", nil)

		sess := jw.NewSession(rr, hr)
		if x := sess; x == nil || x != jw.GetSession(hr) {
			t.Fatal(x)
		}
		if x := len(rr.Result().Cookies()); x != 1 {
			t.Fatal(x)
		}

		r1 := jw.NewRequest(hr)
		if x := sess; x != r1.Session() {
			t.Error(x)
		}
		if x := len(sess.requests); x != 1 {
			t.Error(x)
		}
		if x := sess.requests[0]; x != r1 {
			t.Error(x)
		}

		jw.recycle(r1)
		r1 = nil
		sess.deadline = time.Now()
		if x := jw.SessionCount(); x != 1 {
			t.Error(x)
		}

		go jw.ServeWithTimeout(time.Millisecond)
		// The maintenance ticker reaps the expired session; it floors at 10ms for
		// a 1ms timeout, so advancing the fake clock a full second guarantees
		// several reap cycles, then we let the Serve loop settle.
		time.Sleep(time.Second)
		synctest.Wait()
		if x := jw.SessionCount(); x != 0 {
			t.Error(x)
		}
	})
}

// TestSession_UnclaimedRequestRecycleKeepsGraceDeadline verifies that finishing the
// bootstrap render Request (unclaimed, because its WebSocket has not connected yet)
// does not immediately expire the freshly issued session, so a slightly-slow client
// keeps the session it was just given.
func TestSession_UnclaimedRequestRecycleKeepsGraceDeadline(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)

	sess := jw.NewSession(rr, hr)
	if sess == nil {
		t.Fatal("expected session")
	}
	r1 := jw.NewRequest(hr)
	if r1.Session() != sess {
		t.Fatal("expected request bound to session")
	}

	// Finish the unclaimed bootstrap request (its WebSocket never connected).
	jw.recycle(r1)

	if sess.isDead() {
		t.Fatal("session expired when an unclaimed bootstrap request finished")
	}
	if got := jw.GetSession(hr); got != sess {
		t.Fatalf("expected session still retrievable within its grace window, got %v", got)
	}
}

// TestSession_ClaimedNonLastLeaveKeepsGrace guards a session-lifecycle invariant:
// a claimed (live WebSocket) request leaving while other requests remain attached
// must still refresh the grace deadline.
//
// Otherwise, when the final request to leave is an unclaimed bootstrap render, its
// branch leaves a long-stale deadline intact and an aged session that had an active
// WebSocket moments ago is reaped instantly instead of getting its documented grace
// window for reconnect.
func TestSession_ClaimedNonLastLeaveKeepsGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, _ := New()
		defer func() {
			jw.Close()
			synctest.Wait()
		}()

		rr := httptest.NewRecorder()
		hr := httptest.NewRequest(http.MethodGet, "/", nil)

		sess := jw.NewSession(rr, hr)
		if sess == nil {
			t.Fatal("expected session")
		}

		// rqA is the live tab: its WebSocket connected, so it is claimed and keeps
		// the session alive while its (creation-time) deadline ages into the past.
		rqA := jw.NewRequest(hr)
		if rqA.Session() != sess {
			t.Fatal("expected rqA bound to session")
		}
		rqA.claimed.Store(true)

		// Let the 1-minute creation grace window elapse. The session stays alive only
		// because rqA is attached (len(requests) > 0), not because of its deadline.
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if sess.isDead() {
			t.Fatal("session should stay alive while a claimed request is attached")
		}

		// rqB is a second tab whose bootstrap rendered but whose WebSocket has not
		// connected yet, so it is unclaimed.
		hr2 := httptest.NewRequest(http.MethodGet, "/2", nil)
		hr2.RemoteAddr = hr.RemoteAddr
		hr2.AddCookie(sess.Cookie())
		rqB := jw.NewRequest(hr2)
		if rqB.Session() != sess {
			t.Fatal("expected rqB bound to session")
		}

		// The live tab's WebSocket ends first, while rqB is still attached: this is a
		// claimed request leaving non-last, so delRequest must still refresh the grace.
		rqA.killSession()

		// The second tab's bootstrap then finishes before its WebSocket connects:
		// an unclaimed request leaving last, which leaves the refreshed deadline intact.
		rqB.killSession()

		// A session that had a live WebSocket connection moments ago keeps its grace
		// window so the client can reconnect.
		if sess.isDead() {
			t.Fatal("session was reaped instantly: claimed request leaving non-last lost the grace window")
		}
		if got := jw.GetSession(hr); got != sess {
			t.Fatalf("expected session retrievable within grace window, got %v", got)
		}
	})
}

func TestSession_GetSessionExpiredBeforeCleanup(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)

	sess := jw.NewSession(rr, hr)
	if sess == nil {
		t.Fatal("expected session")
	}
	if got := jw.GetSession(hr); got != sess {
		t.Fatalf("expected live session, got %v", got)
	}

	// Expire the session without running maintenance cleanup.
	sess.mu.Lock()
	sess.deadline = time.Now().Add(-time.Second)
	sess.mu.Unlock()

	if got := jw.SessionCount(); got != 1 {
		t.Fatalf("expected expired session to still be in map before cleanup, got %d", got)
	}
	if got := jw.GetSession(hr); got != nil {
		t.Fatalf("expected expired session to be ignored by GetSession, got %v", got)
	}
}

func TestSession_GetSessionRejectsCookieValueWithTail(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := jw.NewSession(rr, hr)
	if sess == nil {
		t.Fatal("expected session")
	}

	malformed := httptest.NewRequest(http.MethodGet, "/", nil)
	malformed.RemoteAddr = hr.RemoteAddr
	malformed.AddCookie(&http.Cookie{
		Name:  jw.CookieName,
		Value: sess.CookieValue() + "/junk",
	})
	if got := jw.GetSession(malformed); got != nil {
		t.Fatalf("GetSession accepted a cookie value with trailing data: got %v", got)
	}
}

func TestSession_CloseDetachesRequestSession(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	go jw.ServeWithTimeout(time.Second)

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := jw.NewSession(rr, hr)
	if sess == nil {
		t.Fatal("expected session")
	}
	sess.Set("foo", "bar")

	rq := jw.NewRequest(hr)
	if rq.Session() != sess {
		t.Fatal("expected request session association")
	}

	cookie := sess.Close()
	if cookie == nil || cookie.MaxAge != -1 {
		t.Fatalf("expected delete cookie, got %#v", cookie)
	}
	if got := rq.Session(); got != nil {
		t.Fatalf("expected closed session to detach from request, got %v", got)
	}
	if got := rq.Get("foo"); got != nil {
		t.Fatalf("expected detached request Get to return nil, got %v", got)
	}

	jw.recycle(rq)
}

func TestSession_ReplacesOld(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	go jw.ServeWithTimeout(time.Second)

	is := newTestHelper(t)

	is.Equal(jw.SessionCount(), 0)

	w1 := httptest.NewRecorder()
	h1 := httptest.NewRequest("GET", "/", nil)
	s1 := jw.NewSession(w1, h1)
	is.Equal(jw.GetSession(h1), s1)
	is.Equal(len(w1.Result().Cookies()), 1)
	r1 := jw.NewRequest(h1)
	is.Equal(r1.Session(), s1)
	c1 := w1.Result().Cookies()[0]
	is.Equal(c1.MaxAge, 0)
	is.Equal(c1.Name, s1.cookie.Name)
	is.Equal(c1.Value, s1.CookieValue())
	if s1.isDead() {
		t.Fatal("dead")
	}
	s1.Set("foo", "bar")
	is.Equal(s1.Get("foo"), "bar")
	c1copy := *c1

	is.Equal(jw.SessionCount(), 1)

	w2 := httptest.NewRecorder()
	h2 := httptest.NewRequest("GET", "/", nil)
	s2 := jw.NewSession(w2, h2)
	is.Equal(jw.GetSession(h2), s2)
	is.Equal(len(w2.Result().Cookies()), 1)
	r2 := jw.NewRequest(h2)
	is.Equal(r2.Session(), s2)
	c2 := w2.Result().Cookies()[0]
	is.Equal(c2.MaxAge, 0)
	is.Equal(c2.Name, s2.cookie.Name)
	is.Equal(c2.Value, s2.CookieValue())
	if s2.isDead() {
		t.Fatal("dead")
	}

	is.Equal(jw.SessionCount(), 2)
	if s1 == s2 {
		t.Fatal("identical")
	}
	if c1.Value == c2.Value {
		t.Fatal("same value")
	}

	w4 := httptest.NewRecorder()
	h4 := httptest.NewRequest("GET", "/", nil)
	h4.AddCookie(&c1copy)
	r4 := jw.NewRequest(h4)
	is.Equal(r4.Session(), s1)
	is.Equal(jw.GetSession(h4), s1)
	is.Equal(len(w4.Result().Cookies()), 0)

	jw.recycle(r4)

	w3 := httptest.NewRecorder()
	h3 := httptest.NewRequest("GET", "/", nil)
	h3.AddCookie(&c1copy)
	s3 := jw.NewSession(w3, h3)
	is.True(s3 != nil)
	is.Equal(jw.GetSession(h3), s3)
	is.Equal(len(w3.Result().Cookies()), 1)
	c3 := w3.Result().Cookies()[0]
	is.Equal(c3.MaxAge, 0)
	is.Equal(c3.Name, s3.cookie.Name)
	is.Equal(c3.Value, s3.cookie.Value)
	is.True(!s3.isDead())

	is.Equal(jw.SessionCount(), 2)
	is.True(s1 != s3)
	is.True(c1.Value != c3.Value)

	is.True(s1.isDead())
	is.True(!s2.isDead())
	is.True(!s3.isDead())
	is.Equal(s1.Get("foo"), nil)
	is.Equal(s1.Cookie().MaxAge, -1)

	h5 := httptest.NewRequest("GET", "/", nil)
	h5.AddCookie(&c1copy)
	if x := jw.GetSession(h5); x != nil {
		t.Error(x)
	}
}
