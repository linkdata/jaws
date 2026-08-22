package jaws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newSessionTestRequest(sess *Session, path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.AddCookie(sess.Cookie())
	return r
}

func waitTestRequestReady(t *testing.T, tr *TestRequest) {
	t.Helper()
	select {
	case <-tr.ReadyCh:
	case <-tr.DoneCh:
		t.Fatal("Request finished before becoming ready")
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for Request loop")
	}
}

func stopTestRequest(t *testing.T, tr *TestRequest) {
	t.Helper()
	select {
	case <-tr.DoneCh:
		return
	default:
	}
	tr.Close()
	select {
	case <-tr.DoneCh:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for Request loop")
	}
}

func requireSessionCounts(t *testing.T, jw *Jaws, sessions, active int) {
	t.Helper()
	if got := jw.SessionCount(); got != sessions {
		t.Errorf("SessionCount() = %d, want %d", got, sessions)
	}
	if got := jw.ActiveSessionCount(); got != active {
		t.Errorf("ActiveSessionCount() = %d, want %d", got, active)
	}
}

func TestJaws_SessionCountTag(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricSessions)
	})
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag())

	first := jw.NewSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/first", nil))
	if first == nil {
		t.Fatal("NewSession returned nil")
	}
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag())

	first.Close()
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag())

	expired := jw.NewSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/expired", nil))
	if expired == nil {
		t.Fatal("NewSession returned nil")
	}
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag())
	expired.mu.Lock()
	expired.deadline = time.Now().Add(-time.Second)
	expired.mu.Unlock()
	jw.maintenance(time.Hour)
	if got := jw.SessionCount(); got != 0 {
		t.Fatalf("SessionCount() = %d, want 0", got)
	}
	requireDirtyTags(t, jw, jw.SessionCountTag())
}

func TestJaws_ActiveSessionCount(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricActiveSessions)
	})
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveSessionCountTag())

	firstSession := jw.NewSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/first", nil))
	secondSession := jw.NewSession(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/second", nil))
	if firstSession == nil || secondSession == nil {
		t.Fatal("NewSession returned nil")
	}
	requireSessionCounts(t, jw, 2, 0)
	waitingRequest := newSessionTestRequest(firstSession, "/waiting")
	waiting := jw.newRequest(waitingRequest)
	if got := waiting.Session(); got != firstSession {
		t.Fatalf("pending Request Session() = %p, want %p", got, firstSession)
	}
	requireSessionCounts(t, jw, 2, 0)
	if got := jw.UseRequest(waiting.JawsKey, waitingRequest); got != waiting {
		t.Fatalf("UseRequest() = %p, want %p", got, waiting)
	}
	requireSessionCounts(t, jw, 2, 0)
	jw.mu.Lock()
	jw.retireNonRunningRequestLocked(waiting)
	jw.mu.Unlock()

	firstTab := NewTestRequest(jw, newSessionTestRequest(firstSession, "/first-tab"))
	if firstTab == nil {
		t.Fatal("NewTestRequest returned nil")
	}
	t.Cleanup(func() { stopTestRequest(t, firstTab) })
	waitTestRequestReady(t, firstTab)
	requireSessionCounts(t, jw, 2, 1)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveSessionCountTag())

	secondTab := NewTestRequest(jw, newSessionTestRequest(firstSession, "/second-tab"))
	if secondTab == nil {
		t.Fatal("NewTestRequest returned nil")
	}
	t.Cleanup(func() { stopTestRequest(t, secondTab) })
	waitTestRequestReady(t, secondTab)
	requireSessionCounts(t, jw, 2, 1)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw)

	thirdTab := NewTestRequest(jw, newSessionTestRequest(secondSession, "/third-tab"))
	if thirdTab == nil {
		t.Fatal("NewTestRequest returned nil")
	}
	t.Cleanup(func() { stopTestRequest(t, thirdTab) })
	waitTestRequestReady(t, thirdTab)
	requireSessionCounts(t, jw, 2, 2)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveSessionCountTag())

	stopTestRequest(t, firstTab)
	requireSessionCounts(t, jw, 2, 2)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw)

	stopTestRequest(t, secondTab)
	requireSessionCounts(t, jw, 2, 1)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveSessionCountTag())

	secondSession.Close()
	// The first Session remains registered during its disconnect grace period.
	requireSessionCounts(t, jw, 1, 0)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveSessionCountTag())
	stopTestRequest(t, thirdTab)
}

func TestJaws_ActiveSessionCountAutoSession(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.AutoSession = true
		jw.StatusMetrics.Store(StatusMetricSessions | StatusMetricActiveSessions)
	})
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag(), jw.ActiveSessionCountTag())

	server := httptest.NewServer(jw)
	defer server.Close()
	initial := httptest.NewRequest(http.MethodGet, server.URL+"/", nil)
	initial.RemoteAddr = "127.0.0.1:1"
	rq := jw.NewRequest(httptest.NewRecorder(), initial)
	if rq == nil {
		t.Fatal("NewRequest returned nil")
	}
	connected := make(chan *Session, 1)
	rq.SetConnectFn(func(rq *Request) error {
		connected <- rq.Session()
		return nil
	})

	conn := dialJawsRequest(t, server.URL, rq)
	connectionClosed := false
	defer func() {
		if !connectionClosed {
			if closeErr := conn.CloseNow(); closeErr != nil {
				t.Errorf("closing WebSocket: %v", closeErr)
			}
		}
	}()
	sess := waitForConnectSession(t, connected)
	if sess == nil || rq.Session() != sess {
		t.Fatalf("AutoSession = %p, Request.Session() = %p", sess, rq.Session())
	}
	requireSessionCounts(t, jw, 1, 1)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag(), jw.ActiveSessionCountTag())

	if err := conn.CloseNow(); err != nil {
		t.Errorf("CloseNow: %v", err)
	}
	connectionClosed = true
	waitForRequestCount(t, jw, 0, testTimeout)
	requireSessionCounts(t, jw, 1, 0)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveSessionCountTag())
}
