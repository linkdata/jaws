package jaws

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestJaws_parseIP(t *testing.T) {
	is := is.New(t)
	is.Equal(parseIP(""), nil)
	is.Equal(parseIP("127.0.0.1"), net.IPv4(127, 0, 0, 1))
	is.Equal(parseIP("127.0.0.2:1234"), net.IPv4(127, 0, 0, 2))
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
	id1 := jw.MakeID()
	id2 := jw.MakeID()
	is.True(strings.HasPrefix(id1, "jaws."))
	is.True(len(id1) > 5)
	is.True(id1 != id2)
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
		jw.Broadcast(&Message{})
	}
}

func TestJaws_BroadcastWaitsWhenFull(t *testing.T) {
	is := is.New(t)

	jw := New()
	go jw.ServeWithTimeout(testTimeout)

	subCh := jw.subscribe(0)
	defer jw.unsubscribe(subCh)

	// ensure our sub has been processed
	jw.subCh <- nil
	jw.subCh <- nil

	// send two broadcasts
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case jw.bcastCh <- &Message{}:
	}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case jw.bcastCh <- &Message{}:
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

	subCh1 := jw.subscribe(0)
	defer jw.unsubscribe(subCh1)
	subCh2 := jw.subscribe(0)
	defer jw.unsubscribe(subCh2)
	jw.subCh <- nil
	jw.subCh <- nil

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
	case jw.bcastCh <- &Message{}:
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
	case msg := <-subCh1:
		is.Equal(msg, nil)
	default:
	}
}

func TestJaws_UseRequest(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()

	rq1 := jw.NewRequest(context.Background(), "")
	rq2 := jw.NewRequest(context.Background(), "127.0.0.2:1010")
	is.Equal(jw.Pending(), 2)

	rqfail := jw.UseRequest(-2, "")
	is.Equal(rqfail, nil)
	is.Equal(jw.Pending(), 2)

	rqfail = jw.UseRequest(rq1.JawsKey, "127.0.0.1") // wrong IP, expect blank
	is.Equal(rqfail, nil)
	is.Equal(jw.Pending(), 2)

	rqfail = jw.UseRequest(rq2.JawsKey, "127.0.0.1") // wrong IP, expect .2
	is.Equal(rqfail, nil)
	is.Equal(jw.Pending(), 2)

	rq2ret := jw.UseRequest(rq2.JawsKey, "127.0.0.2:1212") // different port is OK
	is.Equal(rq2, rq2ret)
	is.Equal(jw.Pending(), 1)
	rq1ret := jw.UseRequest(rq1.JawsKey, "")
	is.Equal(rq1, rq1ret)
	is.Equal(jw.Pending(), 0)
}

func TestJaws_CleansUpUnconnected(t *testing.T) {
	const numReqs = 1000
	is := is.New(t)
	jw := New()
	defer jw.Close()

	is.Equal(jw.Pending(), 0)
	for i := 0; i < numReqs; i++ {
		jw.NewRequest(context.Background(), "")
	}
	is.Equal(jw.Pending(), numReqs)

	go jw.ServeWithTimeout(time.Millisecond)

	deadline := time.Now().Add(time.Second)
	lastPending := jw.Pending()
	for jw.Pending() > 0 && time.Now().Before(deadline) {
		if jw.Pending() < lastPending {
			lastPending = jw.Pending()
			deadline = time.Now().Add(time.Second)
		}
	}

	is.Equal(jw.Pending(), 0)
}

func TestJaws_BroadcastsCallable(t *testing.T) {
	jw := New()
	defer jw.Close()
	go jw.Serve()

	jw.SetInner("foo", "bar")
	jw.Remove("foo")
	jw.Insert("foo", "bar", "baz")
	jw.Append("foo", "bar")
	jw.Replace("foo", "bar", "baz")
	jw.SetAttr("foo", "bar", "baz")
	jw.RemoveAttr("foo", "bar")
	jw.SetValue("foo", "bar")
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
		case jw.subCh <- nil:
		default:
		}
	}
	is.Equal(jw.subscribe(1), nil)
}
