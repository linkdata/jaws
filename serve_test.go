package jaws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
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

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
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
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
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

		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
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

		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
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
	rq := jw.NewRequest(initial)
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
