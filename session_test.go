package jaws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestSession_Object(t *testing.T) {
	is := is.New(t)
	sessionId := uint64(0x12345)
	var sess *Session
	sess.Set("foo", "bar") // no effect, sess is nil
	is.Equal(nil, sess.Get("foo"))
	sess = newSession(sessionId, nil, time.Now())
	sess.Set("foo", "bar")
	is.Equal("bar", sess.Get("foo"))
	sess.Set("foo", nil)
	is.Equal(nil, sess.Get("foo"))
	cookie := sess.Cookie("testing")
	is.Equal("testing", cookie.Name)
	is.Equal(JawsKeyString(sessionId), cookie.Value)
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
	var sess *Session
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3" {
			r.RemoteAddr = ""
		}
		rq := jw.NewRequest(context.Background(), r)
		if cookie := rq.EnsureSession(1, 60*60*12); cookie != nil {
			http.SetCookie(w, cookie)
		}
		switch r.URL.Path {
		case "/":
			sess = rq.session
			rq.Set("foo", "bar")
		case "/2":
			is.Equal(rq.Get("foo"), "bar")
			rq.Set("foo", "baz")
			sess.SetExpires(time.Now().Add(time.Hour * -12))
		case "/3":
			is.Equal(rq.Get("foo"), nil)
			cookie := rq.SessionCookie()
			is.True(cookie != nil)
			is.True(cookie.Value != JawsKeyString(sess.sessionID))
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
	is.Equal(cookies[0].Value, JawsKeyString(sess.sessionID))
	is.True(sess != nil)
	is.Equal(sess.Get("foo"), "bar")
	is.Equal(jw.GetSession(cookies[0].Value), sess)

	r2, err := http.NewRequest("GET", srv.URL+"/2", nil)
	if err != nil {
		t.Fatal(err)
	}
	r2.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	is.Equal(sess.Get("foo"), "baz")
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
	is.Equal(sess.Get("foo"), "baz")
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
	is.Equal(sess.Get("foo"), nil)
	is.Equal(sess.Get("bar"), "quux")
	is.True(resp != nil)
}

func TestSession_Cleanup(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	sess := jw.createSession(nil, time.Now().Add(time.Millisecond))
	is.True(sess != nil)
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
	// revive the session
	jw.ensureSession(sess)
	is.Equal(jw.SessionCount(), 1)
}
