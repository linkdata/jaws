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

	sess = newSession("blergh", sessionId, nil, time.Now())
	sess.Set("foo", "bar")
	is.Equal("bar", sess.Get("foo"))
	sess.Set("foo", nil)
	is.Equal(nil, sess.Get("foo"))
	cookie := sess.Cookie()
	is.Equal("blergh", cookie.Name)
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
	var wantSess *Session
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3" {
			r.RemoteAddr = ""
		}
		sess, cookie := jw.EnsureSession(r, 60*5, 60*60)
		if cookie != nil {
			http.SetCookie(w, cookie)
		}
		rq := jw.NewRequest(context.Background(), r)
		is.Equal(sess, rq.Session())
		switch r.URL.Path {
		case "/":
			wantSess = sess
			rq.Set("foo", "bar")
		case "/2":
			is.Equal(rq.Get("foo"), "bar")
			rq.Set("foo", "baz")
			wantSess.SetExpires(time.Now().Add(time.Second))
		case "/3":
			is.Equal(r.RemoteAddr, "")
			is.Equal(rq.Session().IP(), nil)
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
	is.Equal(wantSess.Get("foo"), nil)
	is.Equal(wantSess.Get("bar"), "quux")
	is.True(resp != nil)
}

func TestSession_Delete(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	hr := httptest.NewRequest("GET", "/", nil)
	sess, cookie := jw.EnsureSession(hr, 10, 60)
	hr.AddCookie(cookie)
	is.Equal(jw.GetSession(hr), sess)
	is.True(sess != nil)
	is.Equal(sess.Cookie().Value, cookie.Value)
	is.Equal(jw.SessionCount(), 1)
	sl := jw.Sessions()
	is.Equal(1, len(sl))
	is.Equal(sess, sl[0])

	hr2 := httptest.NewRequest("GET", "/", nil)
	hr2.AddCookie(cookie)
	hr2.RemoteAddr = ""
	sess2 := jw.GetSession(hr2)
	is.Equal(sess2, nil)

	cookie1 := sess.Cookie()
	is.True(cookie1 != nil)
	is.Equal(cookie1.Name, jw.CookieName)
	is.True(cookie1.MaxAge >= 0)
	is.True(cookie1.Expires.After(time.Now()))
	cookiefail := jw.DeleteSession(hr2)
	is.Equal(cookiefail, nil)
	hr.RemoteAddr += "0"
	cookie2 := jw.DeleteSession(hr)
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

	hr := httptest.NewRequest("GET", "/", nil)
	sess, _ := jw.EnsureSession(hr, 1, 2)
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
	// revive the session
	jw.ensureSession(sess)
	is.Equal(jw.SessionCount(), 1)
}
