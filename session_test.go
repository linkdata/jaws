package jaws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
	"nhooyr.io/websocket"
)

func TestSession_Object(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	sessionId := uint64(0x12345)
	var sess *Session
	// Set/Get on nil Session is ignored
	sess.Set("foo", "bar")
	is.Equal(nil, sess.Get("foo"))

	sess = newSession(jw, sessionId, nil)
	sess.Set("foo", "bar")
	is.Equal("bar", sess.Get("foo"))
	sess.Set("foo", nil)
	is.Equal(nil, sess.Get("foo"))
	cookie := sess.Cookie()
	is.Equal(jw.CookieName, cookie.Name)
	is.Equal(JawsKeyString(sessionId), cookie.Value)
	is.Equal(sessionId, sess.ID())
	is.Equal(nil, sess.IP())

	sess.Reload()
}

func TestSession_Use(t *testing.T) {
	is := is.New(t)
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
		rq := jw.NewRequest(context.Background(), r)
		is.Equal(sess, rq.Session())

		switch r.URL.Path {
		case "/":
			wantSess = jw.NewSession(w, r)
			wantSess.Set("foo", "bar")
		case "/2":
			is.Equal(rq.Get("foo"), "bar")
			rq.Set("foo", "baz")
		case "/3":
			is.True(rq.Session() != wantSess)
			is.Equal(rq.Get("foo"), nil)
		case "/4":
			is.Equal(rq.Get("foo"), "baz")
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
	is.Equal(len(cookies), 1)
	is.Equal(cookies[0].Name, jw.CookieName)
	is.True(wantSess != nil)
	is.Equal(cookies[0].Value, wantSess.CookieValue())
	is.True(wantSess != nil)
	is.Equal(wantSess.Get("foo"), "bar")

	r2, err := http.NewRequest("GET", srv.URL+"/2", nil)
	if err != nil {
		t.Fatal(err)
	}
	r2.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	is.Equal(wantSess.Get("foo"), "baz")
	is.True(resp != nil)

	rp, err := http.NewRequest("GET", srv.URL+"/jaws/.ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	rp.AddCookie(cookies[0])
	resp, err = srv.Client().Do(rp)
	if err != nil {
		t.Fatal(err)
	}
	is.True(resp != nil)

	r3, err := http.NewRequest("GET", srv.URL+"/3", nil)
	if err != nil {
		t.Fatal(err)
	}
	r3.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r3)
	if err != nil {
		t.Fatal(err)
	}
	is.Equal(wantSess.Get("foo"), "baz")
	is.True(resp != nil)

	r4, err := http.NewRequest("GET", srv.URL+"/4", nil)
	if err != nil {
		t.Fatal(err)
	}
	r4.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r4)
	if err != nil {
		t.Fatal(err)
	}
	is.True(resp != nil)

	is.Equal(wantSess.Get("foo"), nil)
	is.Equal(wantSess.Get("bar"), "quux")
	wantSess.Clear()
	is.Equal(wantSess.Get("bar"), nil)
}

func TestSession_Delete(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
	defer ts.Close()
	go ts.jw.ServeWithTimeout(time.Second)

	is.True(ts.sess != nil)
	is.Equal(ts.jw.SessionCount(), 1)
	sl := ts.jw.Sessions()
	is.Equal(1, len(sl))
	is.Equal(ts.sess, sl[0])

	// session cookie seems ok
	cookie1 := &ts.sess.cookie
	is.True(cookie1 != nil)
	is.Equal(cookie1.Name, ts.jw.CookieName)

	// trying to get the session from another IP fails
	hr2 := httptest.NewRequest("GET", "/", nil)
	hr2.AddCookie(&ts.sess.cookie)
	hr2.RemoteAddr = "10.5.6.7:89"
	sess := ts.jw.GetSession(hr2)
	is.Equal(sess, nil)

	// accessing from same IP but other port works
	hr2.RemoteAddr = ts.hr.RemoteAddr + "0"
	sess = ts.jw.GetSession(hr2)
	is.Equal(ts.sess, sess)

	rq2 := ts.jw.NewRequest(context.Background(), hr2)
	is.Equal(ts.sess, rq2.Session())

	ts.rq.RegisterEventFn("byebye", func(rq *Request, evt what.What, id, val string) error {
		sess2 := ts.jw.GetSession(rq.Initial)
		is.Equal(ts.sess, sess2)
		cookie2 := sess2.Close()
		is.True(cookie2 != nil)
		is.True(cookie2.MaxAge < 0)
		is.True(cookie2.Expires.IsZero())
		is.Equal(cookie1.Name, cookie2.Name)
		is.Equal(cookie1.Value, cookie2.Value)
		return nil
	})

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusSwitchingProtocols)
	defer conn.Close(websocket.StatusNormalClosure, "")

	ts.rq.Send(&Message{
		Tag:  "byebye",
		What: what.Trigger,
	})

	ctx, cancel := context.WithTimeout(ts.ctx, time.Second)
	defer cancel()

	mt, b, err := conn.Read(ctx)
	is.NoErr(err)
	is.Equal(mt, websocket.MessageText)
	is.Equal(string(b), "-1\n\n")
}

func TestSession_Cleanup(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	rr := httptest.NewRecorder()
	hr := httptest.NewRequest("GET", "/", nil)
	sess := jw.NewSession(rr, hr)
	is.Equal(jw.GetSession(hr), sess)
	is.True(sess != nil)
	is.Equal(len(rr.Result().Cookies()), 1)

	r1 := jw.NewRequest(context.Background(), hr)
	is.True(r1 != nil)
	is.Equal(r1.Session(), sess)
	is.Equal(len(sess.requests), 1)
	is.Equal(sess.requests[0], r1)

	r1.recycle()
	r1 = nil
	sess.deadline = time.Now()
	is.Equal(jw.SessionCount(), 1)

	go jw.ServeWithTimeout(time.Millisecond)
	waited := 0
	for waited < 100 && jw.SessionCount() > 0 {
		waited++
		time.Sleep(time.Millisecond)
	}
	is.Equal(jw.SessionCount(), 0)
}

func TestSession_ReplacesOld(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	go jw.ServeWithTimeout(time.Second)

	is.Equal(jw.SessionCount(), 0)

	w1 := httptest.NewRecorder()
	h1 := httptest.NewRequest("GET", "/", nil)
	s1 := jw.NewSession(w1, h1)
	is.True(s1 != nil)
	is.Equal(jw.GetSession(h1), s1)
	is.Equal(len(w1.Result().Cookies()), 1)
	r1 := jw.NewRequest(context.Background(), h1)
	is.Equal(r1.Session(), s1)
	c1 := w1.Result().Cookies()[0]
	is.Equal(c1.MaxAge, 0)
	is.Equal(c1.Name, s1.cookie.Name)
	is.Equal(c1.Value, s1.CookieValue())
	is.True(!s1.isDead())
	s1.Set("foo", "bar")
	is.Equal(s1.Get("foo"), "bar")
	c1copy := *c1

	is.Equal(jw.SessionCount(), 1)

	w2 := httptest.NewRecorder()
	h2 := httptest.NewRequest("GET", "/", nil)
	s2 := jw.NewSession(w2, h2)
	is.True(s2 != nil)
	is.Equal(jw.GetSession(h2), s2)
	is.Equal(len(w2.Result().Cookies()), 1)
	r2 := jw.NewRequest(context.Background(), h2)
	is.Equal(r2.Session(), s2)
	c2 := w2.Result().Cookies()[0]
	is.Equal(c2.MaxAge, 0)
	is.Equal(c2.Name, s2.cookie.Name)
	is.Equal(c2.Value, s2.CookieValue())
	is.True(!s2.isDead())

	is.Equal(jw.SessionCount(), 2)
	is.True(s1 != s2)
	is.True(c1.Value != c2.Value)

	w4 := httptest.NewRecorder()
	h4 := httptest.NewRequest("GET", "/", nil)
	h4.AddCookie(&c1copy)
	r4 := jw.NewRequest(context.Background(), h4)
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
	is.Equal(jw.GetSession(h5), nil)
}
