package jaws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
	"nhooyr.io/websocket"
)

func TestSession_Object(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	sessionId := uint64(0x12345)
	var sess *Session
	sess.Set("foo", "bar") // no effect, sess is nil
	is.Equal(nil, sess.Get("foo"))

	sess = newSession(jw, sessionId, nil, time.Now().Add(time.Second))
	sess.Set("foo", "bar")
	is.Equal("bar", sess.Get("foo"))
	sess.Set("foo", nil)
	is.Equal(nil, sess.Get("foo"))
	cookie := sess.Cookie()
	is.Equal(jw.CookieName, cookie.Name)
	is.Equal(JawsKeyString(sessionId), cookie.Value)
	is.Equal(cookie.Value, sess.CookieValue())
	when := sess.GetExpires()
	is.True(!when.IsZero())
	sess.SetExpires(when.Add(time.Second))
	is.True(sess.GetExpires().After(when))
	is.Equal(sessionId, sess.ID())
	is.Equal(nil, sess.IP())
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

		sess, cookie := jw.EnsureSession(r)
		if cookie != nil {
			http.SetCookie(w, cookie)
		}
		rq := jw.NewRequest(context.Background(), r)
		is.Equal(sess, rq.Session())

		head := rq.HeadHTML()
		is.True(strings.Contains(string(head), "jawsSession="))

		switch r.URL.Path {
		case "/":
			wantSess = sess
			rq.Set("foo", "bar")
		case "/2":
			is.Equal(rq.Get("foo"), "bar")
			rq.Set("foo", "baz")
			wantSess.SetExpires(time.Now().Add(time.Second * sessionRefreshSeconds / 2))
		case "/3":
			is.True(rq.Session() != wantSess)
			is.Equal(rq.Get("foo"), nil)
			cookie := rq.Session().Cookie()
			is.True(cookie != nil)
			is.True(cookie.Value != JawsKeyString(wantSess.sessionID))
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
	is.Equal(cookies[0].Value, JawsKeyString(wantSess.sessionID))
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
	cookie1 := ts.sess.Cookie()
	is.True(cookie1 != nil)
	is.Equal(cookie1.Name, ts.jw.CookieName)
	is.True(cookie1.MaxAge >= 0)
	is.True(cookie1.Expires.After(time.Now()))

	// trying to get the session from another IP fails
	hr2 := httptest.NewRequest("GET", "/", nil)
	hr2.AddCookie(ts.sess.Cookie())
	hr2.RemoteAddr = "10.5.6.7:89"
	sess := ts.jw.GetSession(hr2)
	is.Equal(sess, nil)

	// accessing from same IP but other port works
	hr2.RemoteAddr = ts.hr.RemoteAddr + "0"
	sess = ts.jw.GetSession(hr2)
	is.Equal(ts.sess, sess)

	ts.rq.RegisterEventFn("byebye", func(rq *Request, id, evt, val string) error {
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
		Elem: "byebye",
		What: "trigger",
	})

	ctx, cancel := context.WithTimeout(ts.ctx, time.Second)
	defer cancel()

	mt, b, err := conn.Read(ctx)
	is.NoErr(err)
	is.Equal(mt, websocket.MessageText)
	is.Equal(string(b), sess.jid()+"\nreload\n")
}

func TestSession_Cleanup(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	hr := httptest.NewRequest("GET", "/", nil)
	sess, _ := jw.EnsureSession(hr)
	is.Equal(jw.GetSession(hr), sess)
	is.True(sess != nil)
	sess.SetExpires(time.Now().Add(time.Millisecond))
	is.Equal(jw.SessionCount(), 1)
	sl := jw.Sessions()
	is.Equal(1, len(sl))
	is.Equal(sess, sl[0])

	go jw.ServeWithTimeout(time.Millisecond)
	waited := 0
	for waited < 100 && jw.SessionCount() > 0 {
		waited++
		time.Sleep(time.Millisecond)
	}
	is.Equal(jw.SessionCount(), 0)
}
