package jaws

import (
	"bufio"
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws/secureheaders"
	"github.com/linkdata/jaws/what"
)

type testBroadcastTagGetter struct{}

func (testBroadcastTagGetter) JawsGetTag(*Request) any {
	return Tag("expanded")
}

func TestCoverage_GenerateHeadAndConvenienceBroadcasts(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err := jw.GenerateHeadHTML("%zz"); err == nil {
		t.Fatal("expected url parse error")
	}
	if err := jw.GenerateHeadHTML("/favicon.ico", "/app.js"); err != nil {
		t.Fatal(err)
	}

	jw.Reload()
	if msg := nextBroadcast(t, jw); msg.What != what.Reload {
		t.Fatalf("unexpected reload msg %#v", msg)
	}
	jw.Redirect("/next")
	if msg := nextBroadcast(t, jw); msg.What != what.Redirect || msg.Data != "/next" {
		t.Fatalf("unexpected redirect msg %#v", msg)
	}
	jw.Alert("info", "hello")
	if msg := nextBroadcast(t, jw); msg.What != what.Alert || msg.Data != "info\nhello" {
		t.Fatalf("unexpected alert msg %#v", msg)
	}

	jw.SetInner("t", template.HTML("<b>x</b>"))
	if msg := nextBroadcast(t, jw); msg.What != what.Inner || msg.Data != "<b>x</b>" {
		t.Fatalf("unexpected set inner msg %#v", msg)
	}
	jw.SetAttr("t", "k", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.SAttr || msg.Data != "k\nv" {
		t.Fatalf("unexpected set attr msg %#v", msg)
	}
	jw.RemoveAttr("t", "k")
	if msg := nextBroadcast(t, jw); msg.What != what.RAttr || msg.Data != "k" {
		t.Fatalf("unexpected remove attr msg %#v", msg)
	}
	jw.SetClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.SClass || msg.Data != "c" {
		t.Fatalf("unexpected set class msg %#v", msg)
	}
	jw.RemoveClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.RClass || msg.Data != "c" {
		t.Fatalf("unexpected remove class msg %#v", msg)
	}
	jw.SetValue("t", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected set value msg %#v", msg)
	}
	jw.Insert("t", "0", "<i>a</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Insert || msg.Data != "0\n<i>a</i>" {
		t.Fatalf("unexpected insert msg %#v", msg)
	}
	jw.Replace("t", "<i>b</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Replace || msg.Data != "<i>b</i>" {
		t.Fatalf("unexpected replace msg %#v", msg)
	}
	jw.Delete("t")
	if msg := nextBroadcast(t, jw); msg.What != what.Delete {
		t.Fatalf("unexpected delete msg %#v", msg)
	}
	jw.Append("t", "<em>c</em>")
	if msg := nextBroadcast(t, jw); msg.What != what.Append || msg.Data != "<em>c</em>" {
		t.Fatalf("unexpected append msg %#v", msg)
	}
	jw.JsCall("t", "fn", `{"a":1}`)
	if msg := nextBroadcast(t, jw); msg.What != what.Call || msg.Data != `fn={"a":1}` {
		t.Fatalf("unexpected jscall msg %#v", msg)
	}
}

func TestBroadcast_ExpandsTagDestBeforeQueue(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tagger := testBroadcastTagGetter{}

	jw.Broadcast(Message{
		Dest: tagger,
		What: what.Inner,
		Data: "x",
	})
	msg := nextBroadcast(t, jw)
	if msg.What != what.Inner || msg.Data != "x" {
		t.Fatalf("unexpected msg %#v", msg)
	}
	if got, ok := msg.Dest.(Tag); !ok || got != Tag("expanded") {
		t.Fatalf("expected expanded Tag destination, got %T(%#v)", msg.Dest, msg.Dest)
	}

	jw.Broadcast(Message{
		Dest: []any{tagger, Tag("extra")},
		What: what.Value,
		Data: "v",
	})
	msg = nextBroadcast(t, jw)
	if msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected msg %#v", msg)
	}
	dest, ok := msg.Dest.([]any)
	if !ok {
		t.Fatalf("expected []any destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
	if len(dest) != 2 || dest[0] != Tag("expanded") || dest[1] != Tag("extra") {
		t.Fatalf("unexpected expanded destination %#v", dest)
	}

	jw.Broadcast(Message{
		Dest: "html-id",
		What: what.Delete,
	})
	msg = nextBroadcast(t, jw)
	if got, ok := msg.Dest.(string); !ok || got != "html-id" {
		t.Fatalf("expected raw html-id destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
}

func TestBroadcast_NoneDestination(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.Broadcast(Message{
		Dest: []any{},
		What: what.Update,
		Data: "x",
	})

	select {
	case msg := <-jw.bcastCh:
		t.Fatalf("expected no pending broadcast, got %T(%#v)", msg.Dest, msg.Dest)
	default:
	}
}

func TestBroadcast_ReturnsWhenClosedAndQueueFull(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.Broadcast(Message{What: what.Alert, Data: "info\nfirst"})
	jw.Close()

	done := make(chan struct{})
	go func() {
		jw.Broadcast(Message{What: what.Alert, Data: "info\nsecond"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked after close")
	}

	msg := nextBroadcast(t, jw)
	if msg.Data != "info\nfirst" {
		t.Fatalf("unexpected queued message %#v", msg)
	}
	select {
	case extra := <-jw.bcastCh:
		t.Fatalf("unexpected extra message after close %#v", extra)
	default:
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestJaws_GenerateHeadHTML_StoresCSPBuiltBySecureHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	extras := []string{
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
		"https://images.example.com/logo.png",
	}
	if err = jw.GenerateHeadHTML(extras...); err != nil {
		t.Fatal(err)
	}

	urls := []*url.URL{
		mustParseURL(t, jw.serveCSS.Name),
		mustParseURL(t, jw.serveJS.Name),
	}
	for _, extra := range extras {
		urls = append(urls, mustParseURL(t, extra))
	}

	wantCSP, err := secureheaders.BuildContentSecurityPolicy(urls)
	if err != nil {
		t.Fatal(err)
	}
	if got := jw.ContentSecurityPolicy(); got != wantCSP {
		t.Fatalf("unexpected CSP:\nwant: %q\ngot:  %q", wantCSP, got)
	}
}

func TestJaws_GenerateHeadHTML_PropagatesResourceParseErrors(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	err = jw.GenerateHeadHTML("https://bad host")
	if err == nil {
		t.Fatal("expected parse error for extra resource URL")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestJaws_SecureHeadersMiddleware_UsesJawsCSP(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err = jw.GenerateHeadHTML(
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css",
		"https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js",
	); err != nil {
		t.Fatal(err)
	}
	wantCSP := jw.ContentSecurityPolicy()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	rr := httptest.NewRecorder()
	jw.SecureHeadersMiddleware(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatal("expected wrapped handler to be called")
	}
	if got := rr.Result().StatusCode; got != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, got)
	}

	hdr := rr.Result().Header
	if got := hdr.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected CSP %q, got %q", wantCSP, got)
	}
	if got := hdr.Get("Strict-Transport-Security"); got != secureheaders.DefaultHeaders.Get("Strict-Transport-Security") {
		t.Fatalf("expected HSTS %q, got %q", secureheaders.DefaultHeaders.Get("Strict-Transport-Security"), got)
	}
}

func TestJaws_SecureHeadersMiddleware_ClonesDefaultHeaders(t *testing.T) {
	orig := secureheaders.DefaultHeaders.Clone()
	defer func() {
		secureheaders.DefaultHeaders = orig
	}()

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	wantCSP := jw.ContentSecurityPolicy()
	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	secureheaders.DefaultHeaders.Set("X-Frame-Options", "SAMEORIGIN")
	secureheaders.DefaultHeaders.Set("Content-Security-Policy", "default-src 'none'")

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	hdr := rr.Result().Header

	if got := hdr.Get("X-Frame-Options"); got != orig.Get("X-Frame-Options") {
		t.Fatalf("expected X-Frame-Options %q, got %q", orig.Get("X-Frame-Options"), got)
	}
	if got := hdr.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("expected CSP %q, got %q", wantCSP, got)
	}
}

func TestJaws_SecureHeadersMiddleware_DoesNotTrustForwardedHeaders(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	mw := jw.SecureHeadersMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if got := rr.Result().Header.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS over HTTP request with forwarded proto, got %q", got)
	}
}

func TestJaws_distributeDirt_AscendingOrder(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := &Request{}
	jw.mu.Lock()
	jw.requests[1] = rq
	jw.dirty[Tag("fourth")] = 4
	jw.dirty[Tag("second")] = 2
	jw.dirty[Tag("fifth")] = 5
	jw.dirty[Tag("first")] = 1
	jw.dirty[Tag("third")] = 3
	jw.dirtOrder = 5
	jw.mu.Unlock()

	if got, want := jw.distributeDirt(), 5; got != want {
		t.Fatalf("distributeDirt() = %d, want %d", got, want)
	}

	rq.mu.RLock()
	got := append([]any(nil), rq.todoDirt...)
	rq.mu.RUnlock()

	want := []any{
		Tag("first"),
		Tag("second"),
		Tag("third"),
		Tag("fourth"),
		Tag("fifth"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dirty tags = %#v, want %#v", got, want)
	}
}

func TestJaws_GenerateHeadHTMLConcurrentWithHeadHTML(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if err := jw.GenerateHeadHTML("/a.js", "/b.css"); err != nil {
					t.Error(err)
					return
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
				var buf bytes.Buffer
				if err := rq.HeadHTML(&buf); err != nil {
					t.Error(err)
				}
				jw.recycle(rq)
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestCoverage_IDAndLookupHelpers(t *testing.T) {
	NextJid = 0
	if a, b := NextID(), NextID(); b <= a {
		t.Fatalf("expected increasing ids, got %d then %d", a, b)
	}
	if got := string(AppendID([]byte("x"))); !strings.HasPrefix(got, "x") || len(got) <= 1 {
		t.Fatalf("unexpected append id result %q", got)
	}
	if got := MakeID(); !strings.HasPrefix(got, "jaws.") {
		t.Fatalf("unexpected id %q", got)
	}

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tmpl := template.Must(template.New("it").Parse(`ok`))
	jw.AddTemplateLookuper(tmpl)
	if got := jw.LookupTemplate("it"); got == nil {
		t.Fatal("expected found template")
	}
	if got := jw.LookupTemplate("missing"); got != nil {
		t.Fatal("expected missing template")
	}
	jw.RemoveTemplateLookuper(nil)
	jw.RemoveTemplateLookuper(tmpl)

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("expected request")
	}
	if got := jw.RequestCount(); got != 1 {
		t.Fatalf("expected one request, got %d", got)
	}
	jw.recycle(rq)
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("expected zero requests, got %d", got)
	}
}

func TestCoverage_CookieParseAndIP(t *testing.T) {
	h := http.Header{}
	h.Add("Cookie", `a=1; jaws=`+JawsKeyString(11)+`; x=2`)
	h.Add("Cookie", `jaws="`+JawsKeyString(12)+`"`)
	h.Add("Cookie", `jaws=not-a-key`)

	ids := getCookieSessionsIds(h, "jaws")
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 12 {
		t.Fatalf("unexpected cookie ids %#v", ids)
	}

	if got := parseIP("127.0.0.1:1234"); !got.IsValid() {
		t.Fatalf("expected parsed host:port ip, got %v", got)
	}
	if got := parseIP("::1"); !got.IsValid() {
		t.Fatalf("expected parsed direct ip, got %v", got)
	}
	if got := parseIP(""); got.IsValid() {
		t.Fatalf("expected invalid ip for empty remote addr, got %v", got)
	}
}

func TestCoverage_NonZeroRandomAndPanic(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	// First random value is zero, second is one.
	zeroThenOne := append(make([]byte, 8), []byte{1, 0, 0, 0, 0, 0, 0, 0}...)
	jw.kg = bufio.NewReader(bytes.NewReader(zeroThenOne))
	if got := jw.nonZeroRandomLocked(); got != 1 {
		t.Fatalf("unexpected non-zero random value %d", got)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on random source read error")
		}
	}()
	jw.kg = bufio.NewReader(errReader{})
	_ = jw.nonZeroRandomLocked()
}

func TestJaws_ServeWithTimeoutBounds(t *testing.T) {
	// Min interval clamp path.
	jwMin, err := New()
	if err != nil {
		t.Fatal(err)
	}
	doneMin := make(chan struct{})
	go func() {
		jwMin.ServeWithTimeout(time.Nanosecond)
		close(doneMin)
	}()
	jwMin.Close()
	select {
	case <-doneMin:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout(min)")
	}

	// Max interval clamp path.
	jwMax, err := New()
	if err != nil {
		t.Fatal(err)
	}
	doneMax := make(chan struct{})
	go func() {
		jwMax.ServeWithTimeout(10 * time.Second)
		close(doneMax)
	}()
	jwMax.Close()
	select {
	case <-doneMax:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout(max)")
	}
}

func TestJaws_ServeWithTimeoutFullSubscriberChannel(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	msgCh := make(chan Message) // unbuffered: always full when nobody receives
	done := make(chan struct{})
	go func() {
		jw.ServeWithTimeout(50 * time.Millisecond)
		close(done)
	}()
	jw.subCh <- subscription{msgCh: msgCh, rq: rq}
	// Ensure ServeWithTimeout has consumed the subscription before broadcast.
	for i := 0; i <= cap(jw.subCh); i++ {
		jw.subCh <- subscription{}
	}
	jw.bcastCh <- Message{What: what.Alert, Data: "x"}

	waitUntil := time.Now().Add(time.Second)
	closed := false
	for !closed && time.Now().Before(waitUntil) {
		select {
		case _, ok := <-msgCh:
			closed = !ok
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if !closed {
		t.Fatal("expected subscriber channel to be closed when full")
	}

	jw.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout exit")
	}
}
