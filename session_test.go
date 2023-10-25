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
	"time"

	"github.com/linkdata/jaws/what"
	"nhooyr.io/websocket"
)

func TestSession_Object(t *testing.T) {
	jw := New()
	defer jw.Close()

	sessionId := uint64(0x12345)
	var sess *Session
	// Set/Get on nil Session is ignored
	sess.Set("foo", "bar")
	if x := sess.Get("foo"); x != nil {
		t.Error(x)
	}

	sess = newSession(jw, sessionId, netip.Addr{})
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
	if JawsKeyString(sessionId) != cookie.Value {
		t.Error(cookie.Value)
	}
	if sessionId != sess.ID() {
		t.Error(sess.ID())
	}
	if sess.IP().IsValid() {
		t.Error(sess.IP())
	}
	sess.Reload()
}

func TestSession_Use(t *testing.T) {
	jw := New()
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

		sess := jw.GetSession(r)
		rq := jw.NewRequest(r)
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
		jw.UseRequest(rq.JawsKey, r)
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
	jw := New()
	defer jw.Close()

	sessionId := uint64(0x12345)
	sess := newSession(jw, sessionId, netip.Addr{})
	if x := sess.Requests(); x != nil {
		t.Error(x)
	}
}

func TestSession_Delete(t *testing.T) {
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	ts := newTestServer()
	defer ts.Close()
	go ts.jw.ServeWithTimeout(time.Second)

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

	ts.rq.Register("byebye", func(e *Element, evt what.What, val string) error {
		sess2 := ts.jw.GetSession(e.Request.Initial)
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

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if x := resp.StatusCode; x != http.StatusSwitchingProtocols {
		t.Error(x)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := wsMsg{Jid: jidForTag(ts.rq, Tag("byebye")), What: what.Input}
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
	case <-tmr.C:
		t.Fatal("timeout")
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
	jw := New()
	defer jw.Close()

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

	r1.recycle()
	r1 = nil
	sess.deadline = time.Now()
	if x := jw.SessionCount(); x != 1 {
		t.Error(x)
	}

	go jw.ServeWithTimeout(time.Millisecond)
	waited := 0
	for waited < 100 && jw.SessionCount() > 0 {
		waited++
		time.Sleep(time.Millisecond)
	}
	if x := jw.SessionCount(); x != 0 {
		t.Error(x)
	}
}

func TestSession_ReplacesOld(t *testing.T) {
	jw := New()
	defer jw.Close()
	go jw.ServeWithTimeout(time.Second)

	is := testHelper{t}

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

	r4.recycle()

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
