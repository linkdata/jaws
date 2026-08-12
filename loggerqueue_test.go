package jaws

import (
	"errors"
	"testing"
	"testing/synctest"
)

type blockingQueueLogger struct {
	started chan error
	release chan struct{}
}

func (*blockingQueueLogger) Info(string, ...any) {}
func (*blockingQueueLogger) Warn(string, ...any) {}

func (l *blockingQueueLogger) Error(_ string, args ...any) {
	l.started <- loggerError(args)
	<-l.release
}

type recordingQueueLogger struct {
	jw        *Jaws
	calls     chan error
	reentrant error
	panicOn   error
	firstDone chan struct{}
	reentered chan struct{}
}

func (*recordingQueueLogger) Info(string, ...any) {}
func (*recordingQueueLogger) Warn(string, ...any) {}

func (l *recordingQueueLogger) Error(_ string, args ...any) {
	err := loggerError(args)
	l.calls <- err
	if err == l.panicOn && l.firstDone != nil {
		<-l.firstDone
	}
	if l.reentrant != nil {
		reentrant := l.reentrant
		l.reentrant = nil
		_ = l.jw.Log(reentrant)
		close(l.reentered)
	}
	if err == l.panicOn {
		panic(err)
	}
}

func loggerError(args []any) error {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			err, _ := args[i+1].(error)
			return err
		}
	}
	return nil
}

func TestLoggerQueueIgnoresIncompleteEntries(t *testing.T) {
	var nilQueue *loggerQueue
	nilQueue.enqueue(nil, errors.New("ignored"))
	nilQueue.close()

	q := newLoggerQueue()
	q.enqueue(nil, errors.New("ignored"))
	q.enqueue(&blockingQueueLogger{}, nil)
	q.close()
	<-q.doneCh
}

func TestJawsLogReturnsBeforeLoggerAndCloseDrains(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		logger := &blockingQueueLogger{
			started: make(chan error, 2),
			release: make(chan struct{}),
		}
		jw.Logger = logger
		wantErr := errors.New("queued")
		returned := make(chan error, 1)

		go func() { returned <- jw.Log(wantErr) }()
		synctest.Wait()
		if got := <-returned; got != wantErr {
			t.Fatalf("Log() = %v, want %v", got, wantErr)
		}
		if got := <-logger.started; got != wantErr {
			t.Fatalf("Logger.Error error = %v, want %v", got, wantErr)
		}

		secondErr := errors.New("queued while logger blocked")
		_ = jw.Log(secondErr)
		jw.Close()
		select {
		case <-jw.loggerQueue.doneCh:
			t.Fatal("logging dispatcher stopped before callback returned")
		default:
		}
		if got := jw.Log(errors.New("after close")); got == nil {
			t.Fatal("Log after Close returned nil")
		}
		close(logger.release)
		synctest.Wait()
		<-jw.loggerQueue.doneCh
		if got := <-logger.started; got != secondErr {
			t.Fatalf("drained Logger.Error error = %v, want %v", got, secondErr)
		}
		select {
		case extra := <-logger.started:
			t.Fatalf("Logger.Error called after Close: %v", extra)
		default:
		}
	})
}

func TestJawsServeWaitsForLoggerDrain(t *testing.T) {
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
		serveDone := make(chan struct{})
		go func() {
			jw.Serve()
			close(serveDone)
		}()
		defer func() {
			jw.Close()
			select {
			case <-logger.release:
			default:
				close(logger.release)
			}
			<-serveDone
		}()
		waitForServeLoop(t, jw)

		wantErr := errors.New("blocks shutdown drain")
		_ = jw.Log(wantErr)
		synctest.Wait()
		got := <-logger.started

		jw.Close()
		synctest.Wait()
		returnedEarly := false
		select {
		case <-serveDone:
			returnedEarly = true
		default:
		}
		close(logger.release)
		synctest.Wait()
		if got != wantErr {
			t.Errorf("Logger.Error error = %v, want %v", got, wantErr)
		}
		if returnedEarly {
			t.Error("Serve returned before the accepted logger callback finished")
		}
		select {
		case <-serveDone:
		default:
			t.Error("Serve did not return after the logger callback finished")
		}
	})
}

func TestJawsLogSerializesRecoversAndAllowsReentry(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	firstErr := errors.New("first panics")
	secondErr := errors.New("second")
	thirdErr := errors.New("reentrant")
	logger := &recordingQueueLogger{
		jw:        jw,
		calls:     make(chan error, 3),
		reentrant: thirdErr,
		panicOn:   firstErr,
		firstDone: make(chan struct{}),
		reentered: make(chan struct{}),
	}
	jw.Logger = logger

	_ = jw.Log(firstErr)
	if got := <-logger.calls; got != firstErr {
		t.Fatalf("first Logger.Error = %v, want %v", got, firstErr)
	}
	_ = jw.Log(secondErr)
	close(logger.firstDone)
	<-logger.reentered
	jw.Close()
	<-jw.loggerQueue.doneCh

	if got := <-logger.calls; got != secondErr {
		t.Fatalf("second Logger.Error = %v, want %v", got, secondErr)
	}
	if got := <-logger.calls; got != thirdErr {
		t.Fatalf("third Logger.Error = %v, want %v", got, thirdErr)
	}
}
