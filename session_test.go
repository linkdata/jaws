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
	var remoteAddr string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3" {
			r.RemoteAddr = ""
		}
		rq := jw.NewRequest(context.Background(), r)
		if cookie := rq.EnsureSession(9, 20); cookie != nil {
			http.SetCookie(w, cookie)
		}
		switch r.URL.Path {
		case "/":
			sess = rq.session
			remoteAddr = r.RemoteAddr
			rq.Set("foo", "bar")
		case "/2":
			is.Equal(rq.Get("foo"), "bar")
			rq.Set("foo", "baz")
			sess.SetExpires(time.Now().Add(time.Second))
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
	is.Equal(jw.GetSession(cookies[0].Value, remoteAddr), sess)
	is.Equal(jw.GetSession(cookies[0].Value, "127.2.3.4:5"), nil)

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

func TestSession_Delete(t *testing.T) {
	const remoteAddr = "127.1.3.5:79"
	is := is.New(t)
	jw := New()
	defer jw.Close()
	remoteIP := parseIP(remoteAddr)
	sess := jw.createSession(remoteIP, time.Now().Add(time.Minute))
	is.True(sess != nil)
	is.Equal(jw.SessionCount(), 1)
	sl := jw.Sessions()
	is.Equal(1, len(sl))
	is.Equal(sess, sl[0])
	cookie1 := sess.Cookie(jw.CookieName)
	is.True(cookie1 != nil)
	is.Equal(cookie1.Name, jw.CookieName)
	is.True(cookie1.MaxAge >= 0)
	is.True(cookie1.Expires.After(time.Now()))
	cookiefail := jw.DeleteSession(cookie1.Value, "")
	is.Equal(cookiefail, nil)
	cookie2 := jw.DeleteSession(cookie1.Value, remoteAddr+"0")
	is.True(cookie2 != nil)
	is.True(cookie2.MaxAge < 0)
	is.True(cookie2.Expires.IsZero())
	is.Equal(cookie1.Name, cookie2.Name)
	is.Equal(cookie1.Value, cookie2.Value)
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
