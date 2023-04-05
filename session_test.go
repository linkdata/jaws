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
	var sess *session
	sess.set("foo", "bar") // no effect, sess is nil
	is.Equal(nil, sess.get("foo"))
	sess = newSession(0, nil)
	sess.set("foo", "bar")
	is.Equal("bar", sess.get("foo"))
	sess.set("foo", nil)
	is.Equal(nil, sess.get("foo"))
}

func TestSession_Use(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	go jw.ServeWithTimeout(time.Second)
	var sess *session
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3" {
			r.RemoteAddr = ""
		}
		rq := jw.NewRequest(context.Background(), r)
		rq.EnableSession()
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, rq.SessionCookie())
			rq.Set("foo", "bar")
			sess = rq.session
		case "/2":
			is.Equal(rq.Get("foo"), "bar")
			rq.Set("foo", "baz")
		case "/3":
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
	is.Equal(cookies[0].Value, JawsKeyString(sess.sessionID))
	is.True(sess != nil)
	is.Equal(sess.get("foo"), "bar")

	r2, err := http.NewRequest("GET", srv.URL+"/2", nil)
	if err != nil {
		t.Fatal(err)
	}
	r2.AddCookie(cookies[0])
	resp, err = srv.Client().Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	is.Equal(sess.get("foo"), "baz")
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
	is.Equal(sess.get("foo"), "baz")
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
	is.Equal(sess.get("foo"), nil)
	is.Equal(sess.get("bar"), "quux")
	is.True(resp != nil)
}
