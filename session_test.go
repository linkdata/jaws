package jaws

import (
	"bufio"
	"context"
	"errors"
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
	"github.com/linkdata/jaws/lib/tag"
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

type reentrantSessionResponseWriter struct {
	*httptest.ResponseRecorder
	jw           *Jaws
	sessionCount int
}

func (w *reentrantSessionResponseWriter) Header() http.Header {
	w.sessionCount = w.jw.SessionCount()
	return w.ResponseRecorder.Header()
}

func TestSession_NewSessionCallsResponseWriterOutsideLock(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	hr := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	rw := &reentrantSessionResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		jw:               jw,
	}
	type result struct {
		sess       *Session
		panicValue any
	}
	done := make(chan result, 1)
	go func() {
		var got result
		defer func() {
			got.panicValue = recover()
			done <- got
		}()
		got.sess = jw.NewSession(rw, hr)
	}()

	var got result
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("NewSession deadlocked while ResponseWriter.Header re-entered Jaws")
	}
	t.Cleanup(jw.Close)
	if got.panicValue != nil {
		t.Fatalf("NewSession panicked while ResponseWriter.Header re-entered Jaws: %v", got.panicValue)
	}
	if got.sess == nil {
		t.Fatal("expected session")
	}
	if rw.sessionCount != 1 {
		t.Fatalf("SessionCount during Header = %d, want 1", rw.sessionCount)
	}
	if count := jw.SessionCount(); count != 1 {
		t.Fatalf("SessionCount after NewSession = %d, want 1", count)
	}
	if sess := jw.GetSession(hr); sess != got.sess {
		t.Fatalf("GetSession after NewSession = %v, want %v", sess, got.sess)
	}
	cookies := rw.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	wantCookie := got.sess.Cookie()
	if cookie := cookies[0]; cookie.Name != wantCookie.Name || cookie.Value != wantCookie.Value {
		t.Fatalf("response cookie = %s=%s, want %s=%s", cookie.Name, cookie.Value, wantCookie.Name, wantCookie.Value)
	}
}

func TestSession_NewSessionWithoutResponseWriter(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	hr := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	sess := jw.NewSession(nil, hr)
	if sess == nil {
		t.Fatal("NewSession returned nil")
	}
	if got := jw.GetSession(hr); got != sess {
		t.Fatalf("GetSession() = %v, want %v", got, sess)
	}
	cookies := hr.Cookies()
	if len(cookies) != 1 || cookies[0].Name != jw.CookieName || cookies[0].Value != sess.CookieValue() {
		t.Fatalf("request cookies = %v, want the new Session cookie", cookies)
	}
}

func TestSession_NewSessionWithoutRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rw := &reentrantSessionResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		jw:               jw,
		sessionCount:     -1,
	}
	if sess := jw.NewSession(rw, nil); sess != nil {
		t.Fatalf("NewSession() = %v, want nil", sess)
	}
	if got := jw.SessionCount(); got != 0 {
		t.Errorf("SessionCount() = %d, want 0", got)
	}
	if rw.sessionCount != -1 {
		t.Errorf("ResponseWriter.Header called for a nil request; SessionCount() = %d", rw.sessionCount)
	}
	if header := rw.Result().Header; len(header) != 0 {
		t.Errorf("response headers = %v, want none", header)
	}
}

func TestSession_NewSessionAfterJawsClose(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Close()

	rw := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	if sess := jw.NewSession(rw, hr); sess != nil {
		t.Fatalf("NewSession() after Close = %v, want nil", sess)
	}
	if cookies := rw.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("response cookies after Close = %v, want none", cookies)
	}
	if cookies := hr.Cookies(); len(cookies) != 0 {
		t.Errorf("request cookies after Close = %v, want none", cookies)
	}
	if got := jw.SessionCount(); got != 0 {
		t.Errorf("SessionCount() after Close = %d, want 0", got)
	}
	if sessions := jw.Sessions(); len(sessions) != 0 {
		t.Errorf("Sessions() after Close = %v, want none", sessions)
	}
	if got := jw.GetSession(hr); got != nil {
		t.Errorf("GetSession() after Close = %v, want nil", got)
	}
}

func TestSession_NewSessionUnlocksAfterInjectedRandomReaderPanic(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.kg = bufio.NewReader(errReader{})

	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
		}()
		jw.NewSession(nil, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	}()
	if panicValue == nil {
		jw.Close()
		t.Fatal("NewSession did not panic when the injected random reader failed")
	}
	if !jw.mu.TryLock() {
		// Release the leaked lock so cleanup itself does not deadlock on a regression.
		jw.mu.Unlock()
		jw.Close()
		t.Fatal("NewSession left Jaws locked after the injected random reader panic")
	}
	jw.mu.Unlock()
	jw.Close()
}

func TestSession_AddCookieRejectsUnavailableSession(t *testing.T) {
	tests := []struct {
		name            string
		makeUnavailable func(*Jaws, *Session)
		wantRegistered  bool
		wantDead        bool
	}{
		{
			name: "expired registered",
			makeUnavailable: func(_ *Jaws, sess *Session) {
				sess.mu.Lock()
				sess.deadline = time.Now().Add(-time.Second)
				sess.mu.Unlock()
			},
			wantRegistered: true,
			wantDead:       true,
		},
		{
			name: "live unregistered",
			makeUnavailable: func(jw *Jaws, sess *Session) {
				sess.mu.Lock()
				sess.deadline = time.Now().Add(24 * time.Hour)
				sess.mu.Unlock()
				jw.deleteSessionIfCurrent(sess)
			},
			wantRegistered: false,
			wantDead:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)

			creationRequest := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
			sess := jw.NewSession(nil, creationRequest)
			if sess == nil {
				t.Fatal("NewSession returned nil")
			}
			tt.makeUnavailable(jw, sess)
			if registered := slices.Contains(jw.Sessions(), sess); registered != tt.wantRegistered {
				t.Fatalf("registered = %t, want %t", registered, tt.wantRegistered)
			}
			if dead := sess.isDead(); dead != tt.wantDead {
				t.Fatalf("dead = %t, want %t", dead, tt.wantDead)
			}

			rw := httptest.NewRecorder()
			hr := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
			sess.addCookie(rw, hr)
			for _, cookie := range hr.Cookies() {
				if cookie.Name == jw.CookieName {
					t.Errorf("request contains unavailable session cookie: %v", cookie)
				}
			}
			for _, cookie := range rw.Result().Cookies() {
				if cookie.Name == jw.CookieName {
					t.Errorf("response contains unavailable session cookie: %v", cookie)
				}
			}
		})
	}
}

type closingSessionResponseWriter struct {
	*httptest.ResponseRecorder
	jw            *Jaws
	closedSession *Session
}

func (w *closingSessionResponseWriter) Header() http.Header {
	if sessions := w.jw.Sessions(); len(sessions) > 0 {
		w.closedSession = sessions[0]
		w.closedSession.Close()
	}
	return w.ResponseRecorder.Header()
}

func TestSessionMiddleware_CloseDuringResponseHeader(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	waitForServeLoop(t, jw)

	rw := &closingSessionResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		jw:               jw,
	}
	hr := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	type result struct {
		rq             *Request
		requestCookies []*http.Cookie
		handlerCalled  bool
		panicValue     any
	}
	done := make(chan result, 1)
	go func() {
		var got result
		defer func() {
			got.panicValue = recover()
			done <- got
		}()
		h := jw.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got.handlerCalled = true
			got.requestCookies = r.Cookies()
			got.rq = jw.newRequest(r)
			w.WriteHeader(http.StatusNoContent)
		}))
		h.ServeHTTP(rw, hr)
	}()

	var got result
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SessionMiddleware deadlocked while ResponseWriter.Header closed the Session")
	}
	t.Cleanup(jw.Close)
	if got.panicValue != nil {
		t.Fatalf("SessionMiddleware panicked while ResponseWriter.Header closed the Session: %v", got.panicValue)
	}
	if !got.handlerCalled {
		t.Fatal("SessionMiddleware did not invoke the wrapped handler")
	}
	if rw.closedSession == nil {
		t.Fatal("ResponseWriter.Header did not observe a published Session")
	}
	if sess := got.rq.Session(); sess != nil {
		t.Errorf("new Request Session() = %v, want nil", sess)
	}
	if requests := rw.closedSession.Requests(); len(requests) != 0 {
		t.Errorf("closed Session Requests() = %v, want none", requests)
	}
	if count := jw.SessionCount(); count != 0 {
		t.Errorf("SessionCount() = %d, want 0", count)
	}
	for _, cookie := range got.requestCookies {
		if cookie.Name == jw.CookieName {
			t.Errorf("wrapped handler request contains closed session cookie: %v", cookie)
		}
	}
	for _, cookie := range rw.Result().Cookies() {
		if cookie.Name == jw.CookieName && cookie.MaxAge >= 0 {
			t.Errorf("response contains live session cookie: %v", cookie)
		}
	}
}

func TestSession_NewSessionReplacesDuplicateCookieSessions(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	t.Cleanup(jw.Close)

	makeRequest := func(remoteAddr string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://example.test/login", nil)
		r.RemoteAddr = remoteAddr
		return r
	}
	newSession := func(remoteAddr string) (sess *Session, cookie *http.Cookie) {
		t.Helper()
		sess = jw.NewSession(httptest.NewRecorder(), makeRequest(remoteAddr))
		if sess == nil {
			t.Fatal("NewSession returned nil")
		}
		cookie = sess.Cookie()
		return
	}
	// attach binds a new Request to the session named by cookie, modelling a
	// browser tab already using it.
	attach := func(remoteAddr string, cookie *http.Cookie, want *Session) (rq *Request) {
		t.Helper()
		r := makeRequest(remoteAddr)
		r.AddCookie(cookie)
		if rq = jw.newRequest(r); rq.Session() != want {
			t.Fatalf("attached request session = %p, want %p", rq.Session(), want)
		}
		return
	}

	const (
		sameIP  = "203.0.113.10:1234"
		otherIP = "198.51.100.20:5678"
	)
	first, firstCookie := newSession(sameIP)
	second, secondCookie := newSession(sameIP)
	other, otherCookie := newSession(otherIP)
	unknown, unknownCookie := newSession(sameIP)
	unknown.Close()
	first.Set("stale", "first")
	second.Set("stale", "second")
	other.Set("keep", "other")

	// A tab on each duplicate session. Rotation must detach and reload both, not
	// just the one named by the first matching cookie.
	firstRequest := attach(sameIP, firstCookie, first)
	secondRequest := attach(sameIP, secondCookie, second)

	r := makeRequest(sameIP)
	// Invalid, unknown, and other-IP cookies stay ignored while every
	// distinct live same-IP session is closed, regardless of duplicates or ordering.
	r.AddCookie(&http.Cookie{Name: jw.CookieName, Value: first.CookieValue() + "/junk"})
	r.AddCookie(unknownCookie)
	r.AddCookie(otherCookie)
	r.AddCookie(firstCookie)
	r.AddCookie(firstCookie) // repeated ID
	r.AddCookie(secondCookie)
	rr := httptest.NewRecorder()
	fresh := jw.NewSession(rr, r)
	if fresh == nil {
		t.Fatal("NewSession returned nil")
	}

	if got := jw.GetSession(r); got != fresh {
		t.Fatalf("GetSession() = %p, want fresh session %p", got, fresh)
	}
	if got := jw.newRequest(r).Session(); got != fresh {
		t.Fatalf("NewRequest().Session() = %p, want fresh session %p", got, fresh)
	}
	for _, old := range []struct {
		name string
		sess *Session
		rq   *Request
	}{
		{name: "first", sess: first, rq: firstRequest},
		{name: "second", sess: second, rq: secondRequest},
	} {
		if got := old.sess.Get("stale"); got != nil {
			t.Errorf("%s old session retained stale data %v", old.name, got)
		}
		if cookie := old.sess.Cookie(); cookie == nil || cookie.MaxAge != -1 {
			t.Errorf("%s old session cookie = %#v, want deletion cookie", old.name, cookie)
		}
		// Close detaches the attached Request and arms a reload on it, so the tab
		// that was using the replaced session reloads instead of keeping it.
		if got := old.rq.Session(); got != nil {
			t.Errorf("%s attached request still bound to session %p", old.name, got)
		}
		old.rq.muQueue.Lock()
		queued := slices.Clone(old.rq.wsQueue)
		old.rq.muQueue.Unlock()
		if len(queued) != 1 || queued[0].What != what.Reload {
			t.Errorf("%s attached request queue = %v, want one Reload", old.name, queued)
		}
	}
	if got := other.Get("keep"); got != "other" {
		t.Fatalf("other-IP session data = %v, want unchanged", got)
	}
	if cookie := other.Cookie(); cookie == nil || cookie.MaxAge < 0 {
		t.Fatalf("other-IP session cookie = %#v, want live cookie", cookie)
	}
	otherRequest := makeRequest(otherIP)
	otherRequest.AddCookie(otherCookie)
	if got := jw.GetSession(otherRequest); got != other {
		t.Fatalf("other-IP GetSession() = %p, want %p", got, other)
	}
	mapped := map[*Session]bool{}
	for _, sess := range jw.Sessions() {
		mapped[sess] = true
	}
	for _, closed := range []struct {
		name string
		sess *Session
	}{
		{name: "first", sess: first},
		{name: "second", sess: second},
		{name: "unknown", sess: unknown},
	} {
		if mapped[closed.sess] {
			t.Errorf("%s session still mapped after being closed", closed.name)
		}
	}
	if !mapped[fresh] || !mapped[other] {
		t.Fatalf("mapped fresh = %v, other-IP = %v, want both retained", mapped[fresh], mapped[other])
	}
	if len(mapped) != 2 {
		t.Fatalf("mapped sessions = %d, want only the fresh and other-IP sessions", len(mapped))
	}
	responseCookies := rr.Result().Cookies()
	if len(responseCookies) != 1 {
		t.Fatalf("response cookies = %d, want one replacement cookie", len(responseCookies))
	}
	if got := responseCookies[0]; got.Name != jw.CookieName || got.Value != fresh.CookieValue() {
		t.Fatalf("replacement cookie = %s=%s, want %s=%s", got.Name, got.Value, jw.CookieName, fresh.CookieValue())
	}
}

func TestSession_NewSessionIgnoresDeadMappedSession(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	makeRequest := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://example.test/login", nil)
		r.RemoteAddr = "203.0.113.10:1234"
		return r
	}
	expired := jw.NewSession(httptest.NewRecorder(), makeRequest())
	if expired == nil {
		t.Fatal("NewSession returned nil")
	}
	expiredCookie := expired.Cookie()
	expired.Set("keep", "expired")
	expired.mu.Lock()
	expired.deadline = time.Now().Add(-time.Second)
	expired.mu.Unlock()
	if !expired.isDead() {
		t.Fatal("expected expired session to be dead")
	}
	if !slices.Contains(jw.Sessions(), expired) {
		t.Fatal("expected dead session to remain mapped before rotation")
	}

	// Do not start the processing loop: without maintenance the cookie is
	// guaranteed to name a dead-but-still-mapped session when NewSession reads it.
	r := makeRequest()
	r.AddCookie(expiredCookie)
	fresh := jw.NewSession(httptest.NewRecorder(), r)
	if fresh == nil {
		t.Fatal("NewSession returned nil")
	}
	if got := expired.Get("keep"); got != "expired" {
		t.Fatalf("dead session data = %v, want unchanged", got)
	}
	if !slices.Contains(jw.Sessions(), expired) {
		t.Fatal("dead session was removed during rotation")
	}
	if got := jw.GetSession(r); got != fresh {
		t.Fatalf("GetSession() = %p, want fresh session %p", got, fresh)
	}
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
		rq := jw.newRequest(r).Writer(&sb)
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

	rq1 := jw.newRequest(hr)
	hr2 := httptest.NewRequest(http.MethodGet, "/2", nil)
	hr2.RemoteAddr = hr.RemoteAddr
	hr2.AddCookie(sess.Cookie())
	rq2 := jw.newRequest(hr2)

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

// TestSession_ProducersSkipRecycled covers a stale Request pointer retained by a
// Session snapshot after the Request finished and detached from the Session. Both
// producers must target only Requests that still belong to the Session.
func TestSession_ProducersSkipRecycled(t *testing.T) {
	th := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	sess := jw.NewSession(rr, hr)
	th.True(sess != nil)

	live := jw.newRequest(hr)
	th.True(live.Session() == sess)

	// Session methods snapshot sess.requests under sess.mu, then process the snapshot
	// after releasing it. This entry models a Request captured in that snapshot that
	// then finished: identities are never reused, so it keeps its own (nonzero) key,
	// but completion detached it from the Session (session == nil), so both producers
	// must skip it.
	finished := &Request{Jaws: jw, JawsKey: key.Key(0x9876)}
	sess.mu.Lock()
	sess.requests = append(sess.requests, finished)
	sess.mu.Unlock()

	// Session.Broadcast targets only the live request, never the finished one.
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
		t.Fatalf("recycled request must not broadcast, got %#v", extra)
	default:
	}

	// Session.Close arms a reload on the live request only, and wakes it with a
	// key-targeted Update. The finished request gets neither.
	closeDone := make(chan struct{})
	go func() {
		sess.Close()
		close(closeDone)
	}()
	got = nextBroadcast(t, jw)
	th.Equal(got.What, what.Update)
	th.Equal(got.Dest, live.JawsKey)
	// deadSession queued the reload before the wake broadcast, so it is visible now.
	live.muQueue.Lock()
	th.Equal(len(live.wsQueue), 1)
	if len(live.wsQueue) == 1 {
		th.Equal(live.wsQueue[0].What, what.Reload)
	}
	live.muQueue.Unlock()
	// The recycled request was skipped entirely: no reload was armed on it.
	finished.muQueue.Lock()
	th.Equal(len(finished.wsQueue), 0)
	finished.muQueue.Unlock()
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Session.Close")
	}
	select {
	case extra := <-jw.bcastCh:
		t.Fatalf("recycled request must not broadcast, got %#v", extra)
	default:
	}

	jw.recycle(live)
}

// TestSessionCloseDoesNotReachLaterRequest covers the window between Session.Close
// snapshotting a Request and detaching it: the Request may finish after Close's
// snapshot but before Close detaches it. Because Request identities are never
// reused, a later client gets a distinct Request that must not receive the old
// Session's reload.
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
	first := jw.newRequest(sessionHTTP)
	stale := jw.newRequest(sessionHTTP)
	if first.Session() != sess || stale.Session() != sess {
		t.Fatal("test requests did not attach to the session")
	}

	// Close snapshots [first, stale], clears sess.requests, then blocks trying
	// to detach first. Observing the cleared Session proves stale is already in
	// Close's private snapshot before it is recycled.
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
	unrelated := jw.newRequest(unrelatedHTTP)
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
		t.Fatalf("old Session close reached later Request: %q", data)
	}
	if !strings.Contains(string(data), marker) {
		t.Fatalf("WebSocket message = %q, want marker %q", data, marker)
	}
}

// TestSessionCloseReloadsAssociatedPendingRequest covers issue #215: closing a
// Session must reload a Request that is associated but whose WebSocket has not
// subscribed yet. The reload is queued on the Request by Session.Close and
// delivered when the WebSocket connects, independent of the (dropped) wake-up
// broadcast.
func TestSessionCloseReloadsAssociatedPendingRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	go jw.Serve()
	waitForServeLoop(t, jw)

	server := httptest.NewServer(jw)
	defer server.Close()

	// The session-associated target is in the pending window: rendered, but its
	// WebSocket has not subscribed. Build it from the server URL so its Host
	// matches the WebSocket Origin (see validateWebSocketOrigin).
	sessionHTTP := httptest.NewRequest(http.MethodGet, server.URL+"/", nil)
	sessionHTTP.RemoteAddr = "127.0.0.1:1"
	sess := jw.NewSession(httptest.NewRecorder(), sessionHTTP)
	target := jw.newRequest(sessionHTTP)
	if target.Session() != sess {
		t.Fatal("target Request was not associated with the session")
	}
	if got := target.loadState(); got != reqPending {
		t.Fatalf("target state = %v, want %v", got, reqPending)
	}

	// A separate control subscription, not attached to the session, is an ordering
	// probe against the serve loop: broadcasts are processed in order.
	controlHTTP := httptest.NewRequest(http.MethodGet, server.URL+"/", nil)
	controlHTTP.RemoteAddr = "127.0.0.2:1"
	control := jw.newRequest(controlHTTP)
	if control.Session() != nil {
		t.Fatal("control Request must not share the session")
	}
	controlCh := jw.subscribe(control, 8)
	if controlCh == nil {
		t.Fatal("control subscription failed")
	}
	waitForServeLoop(t, jw) // ensure control is installed in subs

	sess.Close()

	// Prove the close wake-up was processed while the target had no subscription:
	// once the control marker arrives, the earlier key-targeted Update to the
	// (unsubscribed) target has already been handled and dropped. The unfixed
	// implementation broadcast the reload here and lost it.
	const controlMarker = "control ordering marker"
	jw.Broadcast(wire.Message{Dest: control.JawsKey, What: what.Alert, Data: controlMarker})
	select {
	case msg := <-controlCh:
		if msg.What != what.Alert || msg.Data != controlMarker {
			t.Fatalf("control subscription got %#v, want Alert %q", msg, controlMarker)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for control ordering marker")
	}

	// Now the target's browser opens its WebSocket.
	connected := make(chan struct{})
	target.SetConnectFn(func(*Request) error {
		close(connected)
		return nil
	})

	hdr := http.Header{}
	hdr.Set("Origin", server.URL)
	dialCtx, cancelDial := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancelDial()
	conn, resp, err := websocket.Dial(dialCtx,
		"ws"+strings.TrimPrefix(server.URL, "http")+"/jaws/"+target.JawsKeyString(),
		&websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("WebSocket status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("target Request did not start its WebSocket")
	}

	// A post-connect marker terminates the read loop. Batching is opportunistic, so
	// the reload may arrive in an earlier frame; accumulate frames until the marker
	// and assert exactly one Reload command survived the close.
	const targetMarker = "post-connect marker"
	jw.Broadcast(wire.Message{Dest: target.JawsKey, What: what.Alert, Data: targetMarker})

	readCtx, cancelRead := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancelRead()
	var acc strings.Builder
	for !strings.Contains(acc.String(), targetMarker) {
		mt, data, err := conn.Read(readCtx)
		if err != nil {
			t.Fatalf("reading from target Request: %v (got %q)", err, acc.String())
		}
		if mt != websocket.MessageText {
			t.Fatalf("WebSocket message type = %v, want text", mt)
		}
		acc.Write(data)
	}
	if n := strings.Count(acc.String(), what.Reload.String()+"\t"); n != 1 {
		t.Fatalf("got %d Reload commands, want exactly 1: %q", n, acc.String())
	}
}

// TestSessionCloseReloadsConnectedRequestExactlyOnce closes a Session whose
// Request is already connected and asserts that exactly one Reload is delivered.
// It reads through a post-close marker rather than a single frame, so a delayed
// duplicate reload would be caught.
func TestSessionCloseReloadsConnectedRequestExactlyOnce(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	go jw.Serve()
	waitForServeLoop(t, jw)

	server := httptest.NewServer(jw)
	defer server.Close()

	sessionHTTP := httptest.NewRequest(http.MethodGet, server.URL+"/", nil)
	sessionHTTP.RemoteAddr = "127.0.0.1:1"
	sess := jw.NewSession(httptest.NewRecorder(), sessionHTTP)
	rq := jw.newRequest(sessionHTTP)
	if rq.Session() != sess {
		t.Fatal("Request was not associated with the session")
	}

	connected := make(chan struct{})
	rq.SetConnectFn(func(*Request) error {
		close(connected)
		return nil
	})

	hdr := http.Header{}
	hdr.Set("Origin", server.URL)
	dialCtx, cancelDial := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancelDial()
	conn, resp, err := websocket.Dial(dialCtx,
		"ws"+strings.TrimPrefix(server.URL, "http")+"/jaws/"+rq.JawsKeyString(),
		&websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("WebSocket status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("Request did not start its WebSocket")
	}

	// The Request is now connected and subscribed; close the session and then send
	// a marker to bound the read.
	sess.Close()
	const marker = "post-close marker"
	jw.Broadcast(wire.Message{Dest: rq.JawsKey, What: what.Alert, Data: marker})

	readCtx, cancelRead := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancelRead()
	var acc strings.Builder
	for !strings.Contains(acc.String(), marker) {
		mt, data, err := conn.Read(readCtx)
		if err != nil {
			t.Fatalf("reading from Request: %v (got %q)", err, acc.String())
		}
		if mt != websocket.MessageText {
			t.Fatalf("WebSocket message type = %v, want text", mt)
		}
		acc.Write(data)
	}
	if n := strings.Count(acc.String(), what.Reload.String()+"\t"); n != 1 {
		t.Fatalf("got %d Reload commands, want exactly 1: %q", n, acc.String())
	}
}

// TestServeKeyTargetedUpdateFailFast verifies the overload classification the
// Session.Close wake-up relies on: only the internal nil-destination Update tick
// is droppable, while every addressed Update — tag-targeted or key-targeted — is
// one-shot and must fail-fast an overloaded Request.
func TestServeKeyTargetedUpdateFailFast(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	waitForServeLoop(t, jw)

	// A drained control subscription proves a broadcast was processed: broadcasts
	// are ordered, so once its marker arrives every earlier broadcast is handled.
	control := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	controlCh := jw.subscribe(control, 32)
	if controlCh == nil {
		t.Fatal("control subscription failed")
	}
	waitForServeLoop(t, jw)
	awaitControl := func(marker string) {
		t.Helper()
		jw.Broadcast(wire.Message{Dest: control.JawsKey, What: what.Alert, Data: marker})
		deadline := time.After(time.Second)
		for {
			select {
			case msg := <-controlCh:
				if msg.What == what.Alert && msg.Data == marker {
					return
				}
			case <-deadline:
				t.Fatalf("timeout waiting for control marker %q", marker)
			}
		}
	}
	awaitCancel := func(name string, rq *Request) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for rq.Context().Err() == nil {
			if time.Now().After(deadline) {
				t.Fatalf("%s: Request was not cancelled", name)
			}
			time.Sleep(time.Millisecond)
		}
		if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrRequestOverloaded) {
			t.Fatalf("%s: cause = %v, want it to wrap ErrRequestOverloaded", name, cause)
		}
	}

	// A nil-destination Update is the coalescible dirty-render tick: overflowing it
	// must neither cancel the Request nor kill its subscription.
	dropRq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	dropCh := jw.subscribe(dropRq, 1) // not drained during the overflow below
	if dropCh == nil {
		t.Fatal("drop subscription failed")
	}
	waitForServeLoop(t, jw)
	for i := 0; i < 4; i++ {
		jw.Broadcast(wire.Message{What: what.Update})
	}
	awaitControl("after ordinary updates")
	if cause := context.Cause(dropRq.Context()); cause != nil {
		t.Fatalf("nil-destination Update overload cancelled the Request: %v", cause)
	}
	// Prove the subscription survived (was not killed): drain the buffered tick,
	// then a targeted message must still be delivered on the same channel.
	for drained := false; !drained; {
		select {
		case _, ok := <-dropCh:
			if !ok {
				t.Fatal("nil-destination Update overload killed the subscription")
			}
		default:
			drained = true
		}
	}
	const dropMarker = "drop survives"
	jw.Broadcast(wire.Message{Dest: dropRq.JawsKey, What: what.Alert, Data: dropMarker})
	select {
	case msg, ok := <-dropCh:
		if !ok {
			t.Fatal("nil-destination Update overload killed the subscription")
		}
		if msg.What != what.Alert || msg.Data != dropMarker {
			t.Fatalf("drop subscription got %#v, want Alert %q", msg, dropMarker)
		}
	case <-time.After(time.Second):
		t.Fatal("targeted message did not reach the surviving subscription")
	}

	// A tag-targeted Update is one-shot (no periodic re-send), so overflow must
	// fail-fast.
	tagRq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	tagRq.NewElement(&testUi{}).Tag(tag.Tag("overload-tag"))
	if jw.subscribe(tagRq, 1) == nil { // never drained
		t.Fatal("tag subscription failed")
	}
	waitForServeLoop(t, jw)
	for i := 0; i < 4; i++ {
		jw.Broadcast(wire.Message{Dest: tag.Tag("overload-tag"), What: what.Update})
	}
	awaitCancel("tag-targeted Update", tagRq)

	// A key-targeted Update is the Session.Close wake-up: overflow must fail-fast.
	wakeRq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if jw.subscribe(wakeRq, 1) == nil { // never drained
		t.Fatal("wake subscription failed")
	}
	waitForServeLoop(t, jw)
	for i := 0; i < 4; i++ {
		jw.Broadcast(wire.Message{Dest: wakeRq.JawsKey, What: what.Update})
	}
	awaitCancel("key-targeted Update", wakeRq)
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

	rq2 := ts.jw.newRequest(hr2)
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

		r1 := jw.newRequest(hr)
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

// TestSession_UnclaimedRequestRecycleKeepsGraceDeadline verifies that recycling the
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
	r1 := jw.newRequest(hr)
	if r1.Session() != sess {
		t.Fatal("expected request bound to session")
	}

	// Recycle the unclaimed bootstrap request (its WebSocket never connected).
	jw.recycle(r1)

	if sess.isDead() {
		t.Fatal("session expired when an unclaimed bootstrap request was recycled")
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
		rqA := jw.newRequest(hr)
		if rqA.Session() != sess {
			t.Fatal("expected rqA bound to session")
		}
		rqA.storeState(reqClaimed)

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
		rqB := jw.newRequest(hr2)
		if rqB.Session() != sess {
			t.Fatal("expected rqB bound to session")
		}

		// The live tab's WebSocket ends first, while rqB is still attached: this is a
		// claimed request leaving non-last, so delRequest must still refresh the grace.
		rqA.killSession(rqA.loadState().claimed())

		// The second tab's bootstrap is then recycled before its WebSocket connects:
		// an unclaimed request leaving last, which leaves the refreshed deadline intact.
		rqB.killSession(rqB.loadState().claimed())

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

	rq := jw.newRequest(hr)
	if rq.Session() != sess {
		t.Fatal("expected request session association")
	}

	cookie := sess.Close()
	if cookie == nil || cookie.MaxAge != -1 {
		t.Fatalf("expected delete cookie, got %#v", cookie)
	}
	if got := sess.Get("foo"); got != "bar" {
		t.Fatalf("Session.Get() after Session.Close = %v, want %q", got, "bar")
	}
	sess.Set("after", "close")
	if got := sess.Get("after"); got != "close" {
		t.Fatalf("Session.Set() after Session.Close stored %v, want %q", got, "close")
	}
	if got := rq.Session(); got != nil {
		t.Fatalf("expected closed session to detach from request, got %v", got)
	}
	if got := rq.Get("foo"); got != nil {
		t.Fatalf("expected detached request Get to return nil, got %v", got)
	}

	jw.recycle(rq)
}

func TestSession_CloseDoesNotDeleteSameIDReplacement(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()
	waitForServeLoop(t, jw)

	const sessionID = key.Key(0x1122334455667788)
	remoteIP := netip.MustParseAddr("192.0.2.1")
	stale := newSession(jw, sessionID, remoteIP, false)
	jw.mu.Lock()
	jw.sessions[sessionID] = stale
	jw.mu.Unlock()
	jw.deleteSessionIfCurrent(stale)

	replacement := newSession(jw, sessionID, remoteIP, false)
	jw.mu.Lock()
	jw.sessions[sessionID] = replacement
	jw.mu.Unlock()

	if cookie := stale.Close(); cookie == nil || cookie.MaxAge != -1 {
		t.Fatalf("stale Close cookie = %#v, want deletion cookie", cookie)
	}
	if got := jw.SessionCount(); got != 1 {
		t.Fatalf("SessionCount() = %d, want replacement retained", got)
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	request.RemoteAddr = remoteIP.String() + ":1234"
	request.AddCookie(replacement.Cookie())
	if got := jw.GetSession(request); got != replacement {
		t.Fatalf("GetSession() = %p, want replacement %p", got, replacement)
	}

	replacement.Close()
	if got := jw.SessionCount(); got != 0 {
		t.Fatalf("SessionCount() after current Close = %d, want 0", got)
	}
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
	r1 := jw.newRequest(h1)
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
	r2 := jw.newRequest(h2)
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
	r4 := jw.newRequest(h4)
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
