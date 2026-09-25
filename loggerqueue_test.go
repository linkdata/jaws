package jaws

import (
	"errors"
	"fmt"
	"testing"
)

func TestJawsLogBoundsQueueAndReportsDrops(t *testing.T) {
	for _, closeBeforeDrain := range []bool{false, true} {
		name := "drain"
		if closeBeforeDrain {
			name = "close"
		}
		t.Run(name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			logger := &blockingQueueLogger{
				started: make(chan error, maxQueuedLogs+3),
				release: make(chan struct{}),
			}
			jw.Logger = logger
			released := false
			defer func() {
				if !released {
					close(logger.release)
				}
				jw.Close()
				<-jw.loggerQueue.doneCh
			}()

			first := errors.New("in flight")
			_ = jw.Log(first)
			if got := <-logger.started; got != first {
				t.Fatalf("first Logger.Error = %v, want %v", got, first)
			}

			queued := make([]error, maxQueuedLogs)
			for i := range queued {
				queued[i] = fmt.Errorf("queued %d", i)
				_ = jw.Log(queued[i])
			}
			for range 3 {
				_ = jw.Log(errors.New("discarded"))
			}
			jw.loggerQueue.mu.Lock()
			depth := jw.loggerQueue.depth
			dropped := uint64(0)
			if jw.loggerQueue.dropEntry != nil {
				dropped = jw.loggerQueue.dropEntry.dropped
			}
			jw.loggerQueue.mu.Unlock()
			if depth != maxQueuedLogs || dropped != 3 {
				t.Fatalf("queue depth = %d, dropped = %d; want %d, 3", depth, dropped, maxQueuedLogs)
			}
			if got, want := jw.ErrorCount(), uint64(maxQueuedLogs+4); got != want {
				t.Fatalf("ErrorCount() = %d, want %d", got, want)
			}

			if closeBeforeDrain {
				jw.Close()
				select {
				case <-jw.loggerQueue.doneCh:
					t.Fatal("logging dispatcher stopped before Logger.Error returned")
				default:
				}
			}
			close(logger.release)
			released = true
			for i, want := range queued {
				if got := <-logger.started; got != want {
					t.Fatalf("queued Logger.Error[%d] = %v, want %v", i, got, want)
				}
			}
			if got := <-logger.started; got == nil || got.Error() != "jaws: 3 diagnostics dropped" {
				t.Fatalf("drop summary = %v", got)
			}

			if !closeBeforeDrain {
				later := errors.New("after drain")
				_ = jw.Log(later)
				if got := <-logger.started; got != later {
					t.Fatalf("Logger.Error after drain = %v, want %v", got, later)
				}
			}
			jw.Close()
			<-jw.loggerQueue.doneCh
			wantCount := uint64(maxQueuedLogs + 4)
			if !closeBeforeDrain {
				wantCount++
			}
			if got := jw.ErrorCount(); got != wantCount {
				t.Errorf("ErrorCount() after drain = %d, want %d", got, wantCount)
			}
			select {
			case extra := <-logger.started:
				t.Errorf("unexpected Logger.Error after drain: %v", extra)
			default:
			}
		})
	}
}

type steppedQueueLogger struct {
	started chan error
	release chan struct{}
}

func (*steppedQueueLogger) Info(string, ...any) {}
func (*steppedQueueLogger) Warn(string, ...any) {}

func (l *steppedQueueLogger) Error(_ string, args ...any) {
	l.started <- loggerError(args)
	<-l.release
}

func TestJawsLogReportsDropsBeforeLaterAcceptedReports(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	logger := &steppedQueueLogger{
		started: make(chan error, maxQueuedLogs+3),
		release: make(chan struct{}),
	}
	jw.Logger = logger
	released := false
	defer func() {
		if !released {
			close(logger.release)
		}
		jw.Close()
		<-jw.loggerQueue.doneCh
	}()

	first := errors.New("in flight")
	_ = jw.Log(first)
	if got := <-logger.started; got != first {
		t.Fatalf("first Logger.Error = %v, want %v", got, first)
	}
	queued := make([]error, maxQueuedLogs)
	for i := range queued {
		queued[i] = fmt.Errorf("queued %d", i)
		_ = jw.Log(queued[i])
	}
	_ = jw.Log(errors.New("discarded"))
	logger.release <- struct{}{}
	if got := <-logger.started; got != queued[0] {
		t.Fatalf("queued Logger.Error[0] = %v, want %v", got, queued[0])
	}
	later := errors.New("accepted after drop")
	_ = jw.Log(later)
	close(logger.release)
	released = true
	for i, want := range queued[1:] {
		if got := <-logger.started; got != want {
			t.Fatalf("queued Logger.Error[%d] = %v, want %v", i+1, got, want)
		}
	}
	if got := <-logger.started; got == nil || got.Error() != "jaws: 1 diagnostics dropped" {
		t.Fatalf("drop summary = %v", got)
	}
	if got := <-logger.started; got != later {
		t.Fatalf("later Logger.Error = %v, want %v", got, later)
	}
}
