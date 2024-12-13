package jaws

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestJaws_parseIP(t *testing.T) {
	is := newTestHelper(t)
	is.True(!parseIP("").IsValid())
	is.True(parseIP("192.168.0.1").Compare(netip.MustParseAddr("192.168.0.1")) == 0)
	is.True(parseIP("192.168.0.2:1234").Compare(netip.MustParseAddr("192.168.0.2")) == 0)
	is.True(parseIP("127.0.0.1").Compare(netip.MustParseAddr("127.0.0.1")) == 0)
	is.True(parseIP("::1").Compare(netip.IPv6Loopback()) == 0)
	is.True(equalIP(parseIP("127.0.0.1"), parseIP("::1")))
	is.Equal(equalIP(parseIP("127.0.0.1"), netip.Addr{}), false)
	is.Equal(equalIP(netip.IPv4Unspecified(), netip.Addr{}), false)
	is.Equal(equalIP(netip.IPv6Unspecified(), netip.Addr{}), false)
	is.Equal(equalIP(netip.IPv6Loopback(), netip.Addr{}), false)
	is.Equal(equalIP(netip.Addr{}, netip.Addr{}), true)
}

func TestJaws_getCookieSessionsIds(t *testing.T) {
	const sessId = 1234
	sessCookie := JawsKeyString(sessId)
	is := newTestHelper(t)
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
	is := newTestHelper(t)
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
	is := newTestHelper(t)
	defer func() {
		if recover() == nil {
			is.Fail()
		}
	}()
	maybePanic(errors.New("let's panic!"))
}

func TestJaws_Logger(t *testing.T) {
	is := newTestHelper(t)
	jw := New()
	defer jw.Close()
	var b bytes.Buffer
	jw.Logger = slog.New(slog.NewTextHandler(&b, nil))
	go jw.Serve()
	jw.Log(errors.New("bar"))
	is.True(strings.Contains(b.String(), "msg=bar"))
}

func TestJaws_MustLog(t *testing.T) {
	is := newTestHelper(t)
	jw := New()
	defer jw.Close()

	barErr := errors.New("bar")

	defer func() {
		is.Equal(recover(), barErr)
	}()

	var b bytes.Buffer
	jw.Logger = slog.New(slog.NewTextHandler(&b, nil))
	go jw.Serve()
	jw.MustLog(barErr)
	is.True(strings.Contains(b.String(), "msg=bar"))
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
	th := newTestHelper(t)
	jw := New()
	go jw.ServeWithTimeout(testTimeout)

	subCh := jw.subscribe(jw.NewRequest(nil), 0)
	defer jw.unsubscribe(subCh)

	// ensure our sub has been processed
	jw.subCh <- subscription{}
	jw.subCh <- subscription{}

	// send two broadcasts
	select {
	case <-th.C:
		th.Timeout()
	case jw.bcastCh <- Message{What: what.Reload}:
	}
	select {
	case <-th.C:
		th.Timeout()
	case jw.bcastCh <- Message{What: what.Reload}:
	}

	// read one of the broadcasts, the other is
	// left to fall into the retry loop
	select {
	case <-th.C:
		th.Timeout()
	case <-subCh:
	}

	// wait a bit to make sure it's in the loop
	time.Sleep(time.Millisecond * 5)

	// finally, read the msg
	select {
	case <-th.C:
		th.Timeout()
	case <-subCh:
	}
}

func TestJaws_BroadcastFullClosesChannel(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	go jw.ServeWithTimeout(time.Millisecond)

	doneCh := make(chan struct{})
	failCh := make(chan struct{})

	subCh1 := jw.subscribe(jw.NewRequest(nil), 0)

	defer jw.unsubscribe(subCh1)
	subCh2 := jw.subscribe(jw.NewRequest(nil), 0)
	defer jw.unsubscribe(subCh2)
	jw.subCh <- subscription{}
	jw.subCh <- subscription{}

	go func() {
		select {
		case <-th.C:
			close(failCh)
		case <-subCh2:
			close(doneCh)
		}
	}()

	select {
	case <-th.C:
		th.Timeout()
	case jw.bcastCh <- Message{What: what.Reload}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-failCh:
		th.Timeout()
	case <-doneCh:
	}

	// the order in which subscribers are used is random,
	// so we have to wait a bit to ensure we get processed
	time.Sleep(time.Millisecond * 5)

	select {
	case msg, ok := <-subCh1:
		th.True(!ok)
		th.Equal(msg, Message{})
	default:
	}
}

func TestJaws_UseRequest(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()

	th.Equal(0, jw.RequestCount())

	rq1 := jw.NewRequest(nil)
	th.True(rq1.JawsKey != 0)

	rq2 := jw.NewRequest(&http.Request{RemoteAddr: "10.0.0.2:1010"})
	th.True(rq2.JawsKey != 0)
	th.True(rq1.JawsKey != rq2.JawsKey)
	th.Equal(jw.Pending(), 2)

	rqfail := jw.UseRequest(0, nil) // wrong JawsKey
	th.Equal(rqfail, nil)
	th.Equal(jw.Pending(), 2)

	rqfail = jw.UseRequest(rq1.JawsKey, &http.Request{RemoteAddr: "10.0.0.1:1010"}) // wrong IP, expect blank
	th.Equal(rqfail, nil)
	th.Equal(jw.Pending(), 2)

	rqfail = jw.UseRequest(rq2.JawsKey, &http.Request{RemoteAddr: "10.0.0.1:1010"}) // wrong IP, expect .2
	th.Equal(rqfail, nil)
	th.Equal(jw.Pending(), 2)

	rq2ret := jw.UseRequest(rq2.JawsKey, &http.Request{RemoteAddr: "10.0.0.2:1212"}) // different port is OK
	th.Equal(rq2, rq2ret)
	th.Equal(jw.Pending(), 1)

	rqfail = jw.UseRequest(rq2.JawsKey, &http.Request{RemoteAddr: "10.0.0.2:1214"}) // already claimed
	th.Equal(rqfail, nil)
	th.Equal(jw.Pending(), 1)

	rq1ret := jw.UseRequest(rq1.JawsKey, nil)
	th.Equal(rq1, rq1ret)
	th.Equal(jw.Pending(), 0)
}

func TestJaws_BlockingRandomPanics(t *testing.T) {
	th := newTestHelper(t)
	defer func() {
		if recover() == nil {
			th.Error("expected error")
		}
	}()
	jw := New()
	defer jw.Close()
	jw.kg = bufio.NewReader(&bytes.Buffer{})
	jw.NewRequest(nil)
}

type rawLogger struct {
	w io.Writer
}

var _ slog.Handler = &rawLogger{}

func (h *rawLogger) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *rawLogger) Handle(ctx context.Context, r slog.Record) error {
	_, err := fmt.Fprintf(h.w, "%s\n", r.Message)
	return err
}

func (h *rawLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *rawLogger) WithGroup(name string) slog.Handler {
	return h
}

func TestJaws_CleansUpUnconnected(t *testing.T) {
	const numReqs = 100
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()
	var b bytes.Buffer
	jw.Logger = slog.New(&rawLogger{w: &b})
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	th.Equal(jw.Pending(), 0)
	deadline := time.Now().Add(testTimeout)
	var expectLen int
	for i := 0; i < numReqs; i++ {
		rq := jw.NewRequest(hr)
		if (i % (numReqs / 5)) == 0 {
			rq.NewElement(NewUiDiv(MakeHtmlGetter("meh")))
		}
		err := context.Cause(rq.ctx)
		if err == nil && rq.lastWrite.Before(deadline) {
			err = newErrPendingCancelledLocked(rq, newErrNoWebSocketRequest(rq))
		}
		if err == nil {
			t.Fatal("expected error")
		}
		expectLen += len(err.Error() + "\n")
	}
	th.Equal(jw.Pending(), numReqs)

	go jw.ServeWithTimeout(time.Millisecond)

	lastPending := jw.Pending()
	for jw.Pending() > 0 && time.Now().Before(deadline) {
		if jw.Pending() < lastPending {
			lastPending = jw.Pending()
			deadline = time.Now().Add(time.Second)
		}
	}

	th.Equal(jw.Pending(), 0)

	jw.Close()
	select {
	case <-th.C:
		th.Timeout()
	case <-jw.Done():
	}
	if x := b.Len(); x != expectLen {
		t.Log(b.String())
		th.Equal(b.Len(), expectLen)
	}
}

func TestJaws_RequestWriterExtendsDeadline(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()
	var b bytes.Buffer
	jw.Logger = slog.New(slog.NewTextHandler(&b, nil))
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	rq.lastWrite = time.Now().Add(time.Second)
	lastWrite := rq.lastWrite

	var sb strings.Builder
	rw := rq.Writer(&sb)

	ui := &testUi{renderFn: func(e *Element, w io.Writer, params []any) error {
		w.Write(nil)
		return nil
	}}

	rw.UI(ui)

	th.True(ui.renderCalled > 0)
	th.True(rq.rendering.Load())
	th.Equal(lastWrite, rq.getLastWrite())

	go jw.ServeWithTimeout(time.Millisecond)

	for lastWrite == rq.getLastWrite() {
		select {
		case <-th.C:
			th.Timeout()
		case <-jw.Done():
			th.Error("unexpected close")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if rq.getLastWrite().IsZero() {
		th.Error("last write is zero")
	}
	if rq.getLastWrite() == lastWrite {
		th.Error("last write not modified")
	}
}

func TestJaws_UnconnectedLivesUntilDeadline(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq1 := jw.NewRequest(hr)
	rq1ctx := rq1.Context()
	rq2 := jw.NewRequest(hr)
	rq2.lastWrite = time.Now().Add(-time.Second * 10)
	rq2ctx := rq2.Context()

	th.Equal(jw.Pending(), 2)

	go jw.ServeWithTimeout(time.Second)

	for jw.Pending() > 1 {
		select {
		case <-th.C:
			th.Timeout()
		case <-jw.Done():
			th.Error("unexpected close")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	th.Equal(jw.Pending(), 1)

	jw.Close()
	select {
	case <-th.C:
		th.Timeout()
	case <-jw.Done():
	}

	// neither should have been recycled
	th.Equal(rq1.Jaws, jw)
	th.Equal(rq2.Jaws, jw)

	th.NoErr(context.Cause(rq1ctx))
	if !errors.Is(context.Cause(rq2ctx), errNoWebSocketRequest{}) {
		th.Error(context.Cause(rq2ctx))
	}

}

func TestJaws_BroadcastsCallable(t *testing.T) {
	jw := New()
	defer jw.Close()
	go jw.Serve()

	jw.Delete("foo")
	jw.Insert("foo", "bar", "baz")
	jw.Append("foo", "bar")
	jw.Replace("foo", "bar", "baz")
	jw.Reload()
	jw.Redirect("foo")
	jw.Alert("info", "bar")
	someTags := []any{Tag("tag1"), Tag("tag2")}
	jw.SetInner("regularHtmlId", template.HTML(""))
	jw.SetValue("regularHtmlId", "value")
	jw.SetAttr(someTags, "attribute", "value")
	jw.RemoveAttr(someTags, "attribute")
	jw.SetClass(someTags, "classname")
	jw.RemoveClass(someTags, "classname")
}

func TestJaws_subscribeOnClosedReturnsNil(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	jw.Close()
	<-jw.doneCh
	for len(jw.subCh) < cap(jw.subCh) {
		select {
		case jw.subCh <- subscription{}:
		default:
		}
	}

	th.Equal(jw.subscribe(jw.NewRequest(nil), 1), nil)
}

func TestJaws_GenerateHeadHTML(t *testing.T) {
	const extraScript = "someExtraScript.js?disregard"
	const extraStyle = "http://other.server/someExtraStyle.css"
	const extraImage = "someExtraImage.png"
	const extraFont = "someExtraFont.woff2"
	th := newTestHelper(t)
	jw := New()
	jw.Close()

	th.NoErr(jw.GenerateHeadHTML())
	th.True(strings.Contains(string(jw.headPrefix), JavascriptPath))

	th.NoErr(jw.GenerateHeadHTML(extraScript, extraStyle, extraImage, extraFont))
	th.True(strings.Contains(string(jw.headPrefix), JavascriptPath))
	th.True(strings.Contains(string(jw.headPrefix), extraScript))
	th.True(strings.Contains(string(jw.headPrefix), extraStyle))
	th.True(strings.Contains(string(jw.headPrefix), extraImage))
	th.True(strings.Contains(string(jw.headPrefix), extraFont))

	th.True(jw.GenerateHeadHTML("\n") != nil)
}

func TestJaws_TemplateLookuper(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	th.Equal(rq.Jaws.LookupTemplate("nosuchtemplate"), nil)
	th.Equal(rq.Jaws.LookupTemplate("testtemplate"), rq.jw.testtmpl)
	rq.Jaws.RemoveTemplateLookuper(rq.jw.testtmpl)
	th.Equal(rq.Jaws.LookupTemplate("testtemplate"), nil)
}
