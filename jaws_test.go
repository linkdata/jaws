package jaws

import (
	"bufio"
	"bytes"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
)

func TestJaws_parseIP(t *testing.T) {
	is := is.New(t)
	is.Equal(parseIP(""), nil)
	is.True(parseIP("192.168.0.1").Equal(net.IPv4(192, 168, 0, 1)))
	is.True(parseIP("192.168.0.2:1234").Equal(net.IPv4(192, 168, 0, 2)))
	is.True(parseIP("127.0.0.1").Equal(net.IPv4(127, 0, 0, 1)))
	is.True(parseIP("::1").Equal(net.IPv6loopback))
	is.True(equalIP(parseIP("127.0.0.1"), parseIP("::1")))
}

func TestJaws_getCookieSessionsIds(t *testing.T) {
	const sessId = 1234
	sessCookie := JawsKeyString(sessId)
	is := is.New(t)
	is.Equal(getCookieSessionsIds(nil, "meh"), nil)
	is.Equal(getCookieSessionsIds(http.Header{}, "meh"), nil)
	is.Equal(getCookieSessionsIds(http.Header{"Cookie": []string{}}, "meh"), nil)
	is.Equal(getCookieSessionsIds(http.Header{"Cookie": []string{"foo=123"}}, "meh"), nil)
	is.Equal(getCookieSessionsIds(http.Header{"Cookie": []string{"meh=" + sessCookie}}, "meh"), []uint64{sessId})
	is.Equal(getCookieSessionsIds(http.Header{"Cookie": []string{"meh=\"" + sessCookie + "\""}}, "meh"), []uint64{sessId})
}

func TestJaws_MultipleCloseCalls(t *testing.T) {
	jw := New()
	go jw.Serve()
	jw.Close()
	jw.Close()
}

func TestJaws_MakeID(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	go jw.Serve()
	id1 := MakeID()
	id2 := MakeID()
	is.True(strings.HasPrefix(id1, "jaws."))
	is.True(len(id1) > 5)
	is.True(id1 != id2)
}

func TestJaws_maybePanic(t *testing.T) {
	is := is.New(t)
	defer func() {
		if recover() == nil {
			is.Fail()
		}
	}()
	maybePanic(errors.New("let's panic!"))
}

func TestJaws_Logger(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	jw.Logger = log.New(w, "[foo] ", 0)
	go jw.Serve()
	jw.Log(errors.New("bar"))
	w.Flush()
	is.Equal(b.String(), "[foo] bar\n")
}

func TestJaws_MustLog(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	barErr := errors.New("bar")

	defer func() {
		is.Equal(recover(), barErr)
	}()

	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	jw.Logger = log.New(w, "[foo] ", 0)
	go jw.Serve()
	jw.MustLog(barErr)
	w.Flush()
	is.Equal(b.String(), "[foo] bar\n")
	jw.Logger = nil
	jw.MustLog(barErr)
}

func TestJaws_BroadcastDoesntBlockWhenClosed(t *testing.T) {
	jw := New()
	go jw.Serve()
	jw.Close()
	for i := 0; i < cap(jw.bcastCh)+1; i++ {
		jw.Broadcast(Message{})
	}
}

func TestJaws_BroadcastWaitsWhenFull(t *testing.T) {
	is := is.New(t)

	jw := New()
	go jw.ServeWithTimeout(testTimeout)

	subCh := jw.subscribe(nil, 0)
	defer jw.unsubscribe(subCh)

	// ensure our sub has been processed
	jw.subCh <- subscription{}
	jw.subCh <- subscription{}

	// send two broadcasts
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case jw.bcastCh <- Message{What: what.Reload}:
	}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case jw.bcastCh <- Message{What: what.Reload}:
	}

	// read one of the broadcasts, the other is
	// left to fall into the retry loop
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-subCh:
	}

	// wait a bit to make sure it's in the loop
	time.Sleep(time.Millisecond * 5)

	// finally, read the msg
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-subCh:
	}
}

func TestJaws_BroadcastFullClosesChannel(t *testing.T) {
	is := is.New(t)
	jw := New()
	go jw.ServeWithTimeout(time.Millisecond)

	doneCh := make(chan struct{})
	failCh := make(chan struct{})

	subCh1 := jw.subscribe(nil, 0)
	defer jw.unsubscribe(subCh1)
	subCh2 := jw.subscribe(nil, 0)
	defer jw.unsubscribe(subCh2)
	jw.subCh <- subscription{}
	jw.subCh <- subscription{}

	go func() {
		select {
		case <-time.NewTimer(testTimeout).C:
			close(failCh)
		case <-subCh2:
			close(doneCh)
		}
	}()

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case jw.bcastCh <- Message{What: what.Reload}:
	}

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-failCh:
		is.Fail()
	case <-doneCh:
	}

	// the order in which subscribers are used is random,
	// so we have to wait a bit to ensure we get processed
	time.Sleep(time.Millisecond * 5)

	select {
	case msg, ok := <-subCh1:
		is.True(!ok)
		is.Equal(msg, Message{})
	default:
	}
}

func TestJaws_UseRequest(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	is.Equal(0, jw.RequestCount())

	rq1 := jw.NewRequest(nil)
	is.True(rq1.JawsKey != 0)

	rq2 := jw.NewRequest(&http.Request{RemoteAddr: "10.0.0.2:1010"})
	is.True(rq2.JawsKey != 0)
	is.True(rq1.JawsKey != rq2.JawsKey)
	is.Equal(jw.Pending(), 2)

	rqfail := jw.UseRequest(0, nil) // wrong JawsKey
	is.Equal(rqfail, nil)
	is.Equal(jw.Pending(), 2)

	rqfail = jw.UseRequest(rq1.JawsKey, &http.Request{RemoteAddr: "10.0.0.1:1010"}) // wrong IP, expect blank
	is.Equal(rqfail, nil)
	is.Equal(jw.Pending(), 2)

	rqfail = jw.UseRequest(rq2.JawsKey, &http.Request{RemoteAddr: "10.0.0.1:1010"}) // wrong IP, expect .2
	is.Equal(rqfail, nil)
	is.Equal(jw.Pending(), 2)

	rq2ret := jw.UseRequest(rq2.JawsKey, &http.Request{RemoteAddr: "10.0.0.2:1212"}) // different port is OK
	is.Equal(rq2, rq2ret)
	is.Equal(jw.Pending(), 1)

	rq1ret := jw.UseRequest(rq1.JawsKey, nil)
	is.Equal(rq1, rq1ret)
	is.Equal(jw.Pending(), 0)
}

func TestJaws_BlockingRandomPanics(t *testing.T) {
	is := is.New(t)
	defer func() {
		if recover() == nil {
			is.Fail()
		}
	}()
	jw := New()
	defer jw.Close()
	jw.kg = bufio.NewReader(&bytes.Buffer{})
	jw.NewRequest(nil)
	is.Fail()
}

func TestJaws_CleansUpUnconnected(t *testing.T) {
	const numReqs = 1000
	is := is.New(t)
	jw := New()
	defer jw.Close()
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	jw.Logger = log.New(w, "", 0)
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	is.Equal(jw.Pending(), 0)
	deadline := time.Now().Add(testTimeout)
	var expectLen int
	for i := 0; i < numReqs; i++ {
		rq := jw.NewRequest(hr)
		if (i % (numReqs / 10)) == 0 {
			elem := rq.NewElement(NewUiDiv(makeHtmlGetter("meh")))
			for j := 0; j < maxWsQueueLengthPerElement*10; j++ {
				elem.SetInner("foo")
			}
		}
		err := errPendingCancelled(rq, deadline)
		expectLen += len(err.Error() + "\n")
	}
	is.Equal(jw.Pending(), numReqs)

	go jw.ServeWithTimeout(time.Millisecond)

	lastPending := jw.Pending()
	for jw.Pending() > 0 && time.Now().Before(deadline) {
		if jw.Pending() < lastPending {
			lastPending = jw.Pending()
			deadline = time.Now().Add(time.Second)
		}
	}

	is.Equal(jw.Pending(), 0)
	jw.Close()
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-jw.Done():
	}
	w.Flush()
	if b.Len() != expectLen {
		t.Log(b.String())
		is.Equal(b.Len(), expectLen)
	}
}

func TestJaws_BroadcastsCallable(t *testing.T) {
	jw := New()
	defer jw.Close()
	go jw.Serve()

	jw.Remove("foo")
	jw.Insert("foo", "bar", "baz")
	jw.Append("foo", "bar")
	jw.Replace("foo", "bar", "baz")
	jw.Reload()
	jw.Redirect("foo")
	jw.Trigger("foo", "bar")
	jw.Alert("info", "bar")
}

func TestJaws_subscribeOnClosedReturnsNil(t *testing.T) {
	is := is.New(t)
	jw := New()
	jw.Close()
	<-jw.doneCh
	for len(jw.subCh) < cap(jw.subCh) {
		select {
		case jw.subCh <- subscription{}:
		default:
		}
	}

	is.Equal(jw.subscribe(nil, 1), nil)
}

func TestJaws_GenerateHeadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js?disregard"
	const extraStyle = "http://other.server/someExtraStyle.css"
	is := is.New(t)
	jw := New()
	jw.Close()

	jw.GenerateHeadHTML()
	is.True(strings.Contains(string(jw.headPrefix), JavascriptPath))
	jw.GenerateHeadHTML(extraScript, extraStyle)
	is.True(strings.Contains(string(jw.headPrefix), JavascriptPath))
	is.True(strings.Contains(string(jw.headPrefix), extraScript))
	is.True(strings.Contains(string(jw.headPrefix), extraStyle))

	is.True(jw.GenerateHeadHTML("random.crap") != nil)
	is.True(jw.GenerateHeadHTML("\n") != nil)
}
