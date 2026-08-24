package jaws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/staticserve"
)

type maintenanceTestLogger struct {
	mu   sync.Mutex
	errs []error
}

func (*maintenanceTestLogger) Info(string, ...any) {}
func (*maintenanceTestLogger) Warn(string, ...any) {}

func (l *maintenanceTestLogger) Error(_ string, args ...any) {
	for i := 1; i < len(args); i += 2 {
		if err, ok := args[i].(error); ok {
			l.mu.Lock()
			l.errs = append(l.errs, err)
			l.mu.Unlock()
			return
		}
	}
}

func (l *maintenanceTestLogger) loggedErrors() []error {
	l.mu.Lock()
	errs := append([]error(nil), l.errs...)
	l.mu.Unlock()
	return errs
}

type serveReentrantLogger struct {
	jw     *Jaws
	logged chan error
}

func (*serveReentrantLogger) Info(string, ...any) {}
func (*serveReentrantLogger) Warn(string, ...any) {}

func (l *serveReentrantLogger) Error(_ string, args ...any) {
	var logged error
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			logged, _ = args[i+1].(error)
			break
		}
	}
	_ = l.jw.RequestCount()
	l.jw.Broadcast(wire.Message{What: what.Reload})
	l.jw.Broadcast(wire.Message{What: what.Reload})
	l.logged <- logged
	<-l.jw.Done()
}

func newServeReentrantLogger(jw *Jaws) *serveReentrantLogger {
	return &serveReentrantLogger{
		jw:     jw,
		logged: make(chan error, 2),
	}
}

const panickingServeTagValue = "panicking Serve tag String method"

type panickingServeTag struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (tag panickingServeTag) String() string {
	if tag.started != nil {
		tag.started <- struct{}{}
	}
	if tag.release != nil {
		<-tag.release
	}
	panic(panickingServeTagValue)
}

func TestJaws_ServePanicDoesNotWaitForOpenLoggerQueue(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("full tag rendering is enabled in debug and race builds")
	}
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	servePanic := make(chan any, 1)
	go func() {
		defer func() { servePanic <- recover() }()
		jw.Serve()
	}()
	waitForServeLoop(t, jw)

	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	tagValue := panickingServeTag{}
	elem := rq.NewElement(&testUi{})
	rq.TagExpanded(elem, []any{tagValue})
	msgCh := make(chan wire.Message)
	jw.subCh <- subscription{msgCh: msgCh, rq: rq}
	waitForServeLoop(t, jw)
	jw.Broadcast(wire.Message{Dest: tagValue, What: what.Alert})

	select {
	case recovered := <-servePanic:
		if recovered != panickingServeTagValue {
			t.Fatalf("Serve panic = %v, want %q", recovered, panickingServeTagValue)
		}
	case <-time.After(testTimeout):
		t.Fatal("Serve panic cleanup waited for the still-open logger queue")
	}
}

func TestJaws_ServePanicDoesNotWaitForClosingLoggerQueue(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("full tag rendering is enabled in debug and race builds")
	}
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		logger := &blockingQueueLogger{
			started: make(chan error, 1),
			release: make(chan struct{}),
		}
		jw.Logger = logger
		stringRelease := make(chan struct{})
		var releaseLoggerOnce, releaseStringOnce sync.Once
		releaseLogger := func() { releaseLoggerOnce.Do(func() { close(logger.release) }) }
		releaseString := func() { releaseStringOnce.Do(func() { close(stringRelease) }) }
		defer func() {
			jw.Close()
			releaseString()
			releaseLogger()
			synctest.Wait()
		}()

		servePanic := make(chan any, 1)
		go func() {
			defer func() { servePanic <- recover() }()
			jw.Serve()
		}()
		waitForServeLoop(t, jw)

		blockedErr := errors.New("blocks logger queue shutdown")
		_ = jw.Log(blockedErr)
		synctest.Wait()
		if got := <-logger.started; got != blockedErr {
			t.Fatalf("Logger.Error error = %v, want %v", got, blockedErr)
		}

		stringStarted := make(chan struct{}, 1)
		tagValue := panickingServeTag{started: stringStarted, release: stringRelease}
		rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		elem := rq.NewElement(&testUi{})
		rq.Tag(elem, tagValue)
		msgCh := make(chan wire.Message)
		jw.subCh <- subscription{msgCh: msgCh, rq: rq}
		waitForServeLoop(t, jw)
		jw.Broadcast(wire.Message{Dest: tagValue, What: what.Alert})
		synctest.Wait()
		select {
		case <-stringStarted:
		default:
			t.Fatal("Serve did not begin rendering the panicking tag")
		}

		closeDone := make(chan struct{})
		go func() {
			jw.Close()
			close(closeDone)
		}()
		synctest.Wait()
		select {
		case <-closeDone:
		default:
			t.Fatal("Close did not return while Logger.Error was blocked")
		}
		releaseString()
		synctest.Wait()

		var recovered any
		panicObserved := false
		select {
		case recovered = <-servePanic:
			panicObserved = true
		default:
			t.Error("Serve panic cleanup waited for the closing logger queue")
		}
		select {
		case <-jw.loggerQueue.doneCh:
			t.Error("logging dispatcher stopped while Logger.Error was blocked")
		default:
		}

		releaseLogger()
		synctest.Wait()
		if !panicObserved {
			recovered = <-servePanic
		}
		if recovered != panickingServeTagValue {
			t.Fatalf("Serve panic = %v, want %q", recovered, panickingServeTagValue)
		}
		select {
		case <-jw.loggerQueue.doneCh:
		default:
			t.Error("logging dispatcher did not stop after Logger.Error returned")
		}
	})
}

func TestJaws_ServeMaintenanceLoggerCanBroadcastRepeatedly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		logger := newServeReentrantLogger(jw)
		jw.Logger = logger

		serveDone := make(chan struct{})
		go func() {
			jw.ServeWithTimeout(time.Second)
			close(serveDone)
		}()
		defer func() {
			jw.Close()
			<-serveDone
		}()
		waitForServeLoop(t, jw)

		rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		// Maintenance uses whole-second samples and expires only after the timeout.
		time.Sleep(3 * time.Second)
		synctest.Wait()

		select {
		case logged := <-logger.logged:
			if logged != context.Cause(rq.Context()) || !errors.Is(logged, ErrNoWebSocketRequest) {
				t.Errorf("logged error = %v, want Request cancellation cause", logged)
			}
		default:
			t.Error("Logger.Error did not complete repeated Broadcast calls")
		}
		waitForServeLoop(t, jw)
		select {
		case extra := <-logger.logged:
			t.Errorf("unexpected duplicate log: %v", extra)
		default:
		}

		jw.Close()
		<-serveDone
	})
}

func TestJaws_ServeOverloadLoggerCanBroadcastRepeatedly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		logger := newServeReentrantLogger(jw)
		jw.Logger = logger
		serveDone := make(chan struct{})
		go func() {
			jw.ServeWithTimeout(time.Hour)
			close(serveDone)
		}()
		defer func() {
			jw.Close()
			<-serveDone
		}()
		waitForServeLoop(t, jw)

		rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		msgCh := make(chan wire.Message)
		jw.subCh <- subscription{msgCh: msgCh, rq: rq}
		waitForServeLoop(t, jw)

		jw.Broadcast(wire.Message{What: what.Alert, Data: "overload"})
		synctest.Wait()
		select {
		case logged := <-logger.logged:
			if logged != context.Cause(rq.Context()) || !errors.Is(logged, ErrRequestOverloaded) {
				t.Errorf("logged error = %v, want Request cancellation cause", logged)
			}
		default:
			t.Error("Logger.Error did not complete repeated Broadcast calls")
		}
		waitForServeLoop(t, jw)

		jw.Close()
		<-serveDone
		select {
		case extra := <-logger.logged:
			t.Errorf("unexpected duplicate log: %v", extra)
		default:
		}
	})
}

func TestJaws_MaintenanceRetiresExpiredRequestOnce(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	logger := &maintenanceTestLogger{}
	jw.Logger = logger
	initial := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.newRequest(initial)
	key := rq.JawsKey
	jw.runtimeSeconds.Store(rq.lastWriteSeconds.Load() + 2)

	jw.maintenance(time.Second)
	awaitTestLoggerQueue(t, jw)

	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("RequestCount after maintenance = %d, want 0", got)
	}
	if got := jw.Pending(); got != 0 {
		t.Fatalf("Pending after maintenance = %d, want 0", got)
	}
	if claimed := jw.UseRequest(key, initial); claimed != nil {
		t.Fatalf("expired Request remained claimable as %v", claimed)
	}
	if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrNoWebSocketRequest) {
		t.Fatalf("cancellation cause = %v, want ErrNoWebSocketRequest", cause)
	}
	logged := logger.loggedErrors()
	if len(logged) != 1 {
		t.Fatalf("maintenance logged %d errors, want 1: %v", len(logged), logged)
	}
	if !errors.Is(logged[0], ErrNoWebSocketRequest) {
		t.Fatalf("logged error = %v, want ErrNoWebSocketRequest", logged[0])
	}
}

type testMux struct {
	m map[string]http.Handler
}

func (tm *testMux) Handle(uri string, handler http.Handler) {
	if tm.m == nil {
		tm.m = make(map[string]http.Handler)
	}
	tm.m[uri] = handler
}

type testSetupper struct{}

func (ts *testSetupper) JawsSetupFunc(jw *Jaws, handleFn HandleFunc, prefix string) (urls []*url.URL, err error) {
	ss, _ := staticserve.New("foo.txt", []byte("foo"))
	u, _ := url.Parse(ss.Name)
	urls = append(urls, u)
	handleFn(path.Join(prefix, ss.Name), ss)
	return
}

func TestJaws_Setup(t *testing.T) {
	const prefix = "/static"
	const extraStyle = "someExtraStyle.css"
	ss1 := staticserve.Must("favicon.png", []byte("Hello"))
	ss2 := staticserve.Must("1.txt", []byte("Hello"))
	ss3 := staticserve.Must("2.txt", []byte("Hello"))
	u, _ := url.Parse("relative")
	ts := &testSetupper{}

	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	if err := jw.Setup(mux.Handle, prefix, extraStyle, ss1, u, ts.JawsSetupFunc, []*staticserve.StaticServe{ss2, ss3}); err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 4 {
		t.Log(len(mux.m))
		t.Error(mux.m)
	}
	if _, ok := mux.m["GET "+path.Join(prefix, ss1.Name)]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
	if _, ok := mux.m["GET "+path.Join(prefix, ss2.Name)]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
	if _, ok := mux.m["GET "+path.Join(prefix, ss3.Name)]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
	var barePatterns []string
	for pattern := range mux.m {
		if !strings.HasPrefix(pattern, "GET ") {
			barePatterns = append(barePatterns, pattern)
		}
	}
	if len(barePatterns) != 1 {
		t.Fatalf("expected 1 bare path pattern, got %d: %#v", len(barePatterns), mux.m)
	}
	if got := barePatterns[0]; !strings.HasPrefix(got, prefix+"/foo.") || !strings.HasSuffix(got, ".txt") {
		t.Errorf("unexpected setupfunc pattern: %q", got)
	}
	if x := jw.FaviconURL(); x != path.Join(prefix, ss1.Name) {
		t.Error(x)
	}
}

func TestJaws_SetupURLExtraCanBeReusedWithDifferentPrefixes(t *testing.T) {
	u, err := url.Parse("app.js")
	if err != nil {
		t.Fatal(err)
	}

	jw1, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw1.Close()
	if err = jw1.Setup(nil, "/one", u); err != nil {
		t.Fatal(err)
	}

	jw2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw2.Close()
	if err = jw2.Setup(nil, "/two", u); err != nil {
		t.Fatal(err)
	}

	if got := jw2.headPrefix; !strings.Contains(got, `/two/app.js`) {
		t.Fatalf("second Setup head = %q, want reused URL extra under /two", got)
	}
	if got := u.String(); got != "app.js" {
		t.Fatalf("Setup mutated URL extra: got %q, want %q", got, "app.js")
	}
}

func TestJaws_SetupRelativePrefixYieldsAbsoluteURL(t *testing.T) {
	// A relative, non-empty prefix must still produce a head URL that matches the
	// always-absolute handler pattern; a relative URL would resolve against the
	// current page and 404 on any non-root page.
	ss := staticserve.Must("favicon.png", []byte("Hello"))

	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	if err := jw.Setup(mux.Handle, "static", ss); err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	var pattern string
	for p := range mux.m {
		pattern = p
	}
	headURL := jw.FaviconURL()
	if !strings.HasPrefix(headURL, "/static/favicon.") {
		t.Errorf("head URL is not absolute: %q", headURL)
	}
	if want := "GET " + headURL; pattern != want {
		t.Errorf("handler pattern %q does not match head URL: want %q", pattern, want)
	}
}

func TestJaws_SetupDoesNotPrefixExternalOriginURL(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := new(headWarningLogger)
	jw.Logger = logger

	const rawURL = "https://cdn.example.test"
	if err = jw.Setup(nil, "/static", rawURL); err != nil {
		t.Fatal(err)
	}

	if got := jw.ContentSecurityPolicy(); !strings.Contains(got, "connect-src 'self' "+rawURL) {
		t.Fatalf("Setup CSP = %q, want unmodified external origin %q", got, rawURL)
	}
	if len(logger.urls) != 1 || logger.urls[0] != rawURL {
		t.Fatalf("warning URLs = %q, want [%q]", logger.urls, rawURL)
	}
}

// TestJaws_SetupDoesNotPrefixProtocolRelativeURL guards the Host=="" clause of
// makeAbsPath specifically: a protocol-relative URL carries a host but no scheme,
// so a scheme-only check (such as url.URL.IsAbs) would still prefix and corrupt it.
func TestJaws_SetupDoesNotPrefixProtocolRelativeURL(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const rawURL = "//cdn.example.test/app.js"
	if err = jw.Setup(nil, "/static", rawURL); err != nil {
		t.Fatal(err)
	}

	if got := jw.headPrefix; !strings.Contains(got, `src="`+rawURL+`"`) {
		t.Fatalf("Setup head HTML = %q, want unmodified protocol-relative URL %q", got, rawURL)
	}
}

func TestJaws_SetupEmptyPrefix(t *testing.T) {
	ss := staticserve.Must("favicon.png", []byte("Hello"))

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	mux := http.NewServeMux()
	if err = jw.Setup(mux.Handle, "", ss); err != nil {
		t.Fatal(err)
	}

	headURL := jw.FaviconURL()
	if want := "/" + ss.Name; headURL != want {
		t.Fatalf("favicon URL = %q, want %q", headURL, want)
	}
	if !strings.Contains(jw.headPrefix, `href="`+headURL+`"`) {
		t.Fatalf("head HTML %q does not contain favicon URL %q", jw.headPrefix, headURL)
	}

	pageURL, err := url.Parse("http://example.test/account/view")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := url.Parse(headURL)
	if err != nil {
		t.Fatal(err)
	}
	resolved := pageURL.ResolveReference(ref)
	if resolved.Path != headURL {
		t.Fatalf("favicon URL %q resolves from nested page to %q", headURL, resolved.Path)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, resolved.String(), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %q = %d, want %d", resolved.Path, rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "Hello" {
		t.Fatalf("GET %q body = %q, want %q", resolved.Path, got, "Hello")
	}
}

func TestJaws_SetupStaticServeEscapesName(t *testing.T) {
	ss := staticserve.Must(`favicon:scheme {asset}#query?percent%\file.png`, []byte("Hello"))

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	mux := http.NewServeMux()
	if err = jw.Setup(mux.Handle, "", ss); err != nil {
		t.Fatal(err)
	}

	headURL := jw.FaviconURL()
	if !strings.HasPrefix(headURL, "/favicon:scheme") {
		t.Fatalf("favicon URL is not slash-rooted: %q", headURL)
	}
	for _, escaped := range []string{"%20", "%7Basset%7D", "%23", "%3F", "%25", "%5C"} {
		if !strings.Contains(headURL, escaped) {
			t.Errorf("favicon URL %q does not contain %q", headURL, escaped)
		}
	}

	u, err := url.Parse(headURL)
	if err != nil {
		t.Fatal(err)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		t.Fatalf("favicon URL parsed with query %q and fragment %q", u.RawQuery, u.Fragment)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, headURL, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %q = %d, want %d", headURL, rr.Code, http.StatusOK)
	}

	wildcardURL := strings.Replace(headURL, "%7Basset%7D", "other", 1)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, wildcardURL, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET wildcard candidate %q = %d, want %d", wildcardURL, rr.Code, http.StatusNotFound)
	}
}

func TestJaws_SetupEmptyPrefixKeepsGenericRelativeURLs(t *testing.T) {
	urlExtra, err := url.Parse("url.css")
	if err != nil {
		t.Fatal(err)
	}
	setupURL, err := url.Parse("setup.css")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		extra any
		want  string
	}{
		{name: "string", extra: "string.css", want: "string.css"},
		{name: "URL", extra: urlExtra, want: "url.css"},
		{name: "SetupFunc", extra: SetupFunc(func(_ *Jaws, _ HandleFunc, _ string) (urls []*url.URL, err error) {
			urls = append(urls, setupURL)
			return
		}), want: "setup.css"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()
			if err = jw.Setup(nil, "", tc.extra); err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(jw.headPrefix, `href="`+tc.want+`"`) {
				t.Fatalf("head HTML %q does not contain relative URL %q", jw.headPrefix, tc.want)
			}
			if strings.Contains(jw.headPrefix, `href="/`+tc.want+`"`) {
				t.Fatalf("head HTML %q slash-rooted relative URL %q", jw.headPrefix, tc.want)
			}
		})
	}
}

func TestJaws_SetupKeepsMethodPattern(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	err := jw.Setup(mux.Handle, "", SetupFunc(func(_ *Jaws, handleFn HandleFunc, _ string) (urls []*url.URL, err error) {
		handleFn("POST\t/custom", http.NotFoundHandler())
		return
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	if _, ok := mux.m["POST\t/custom"]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
}

func TestJaws_SetupSetupFuncBarePathIsUnchanged(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	err := jw.Setup(mux.Handle, "", SetupFunc(func(_ *Jaws, handleFn HandleFunc, _ string) (urls []*url.URL, err error) {
		handleFn("custom", http.NotFoundHandler())
		return
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	if _, ok := mux.m["custom"]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
}

func TestJaws_SetupRejectsUnknownExtra(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	mux := http.NewServeMux()
	err := jw.Setup(mux.Handle, "", 1)
	if err == nil {
		t.Fatal("expected an error for an unsupported extra type")
	}
	if !strings.Contains(err.Error(), "not int") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTestServe_PanicsWhenJawsAlreadyClosed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	rq := jw.newRequest(nil)
	jw.Close()

	defer func() {
		got := recover()
		message, ok := got.(string)
		if !ok || !strings.Contains(message, "Jaws instance is closed") {
			t.Fatalf("panic = %v, want closed-Jaws diagnostic", got)
		}
	}()
	jw.TestServe(rq, func(any) {})
	t.Fatal("TestServe did not panic for a closed Jaws instance")
}
