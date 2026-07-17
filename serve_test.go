package jaws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
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
