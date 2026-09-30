package examples

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
)

func TestSecureSessionHeadersOnSessionLimit(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.MaxSessions = 1
	handler := jw.SecureHeadersMiddleware(jw.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	for _, wantStatus := range []int{http.StatusNoContent, http.StatusServiceUnavailable} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != wantStatus {
			t.Fatalf("status = %d, want %d", response.Code, wantStatus)
		}
		if got := response.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("X-Frame-Options = %q, want DENY", got)
		}
		if got := response.Header().Get("Content-Security-Policy"); got == "" {
			t.Fatal("missing Content-Security-Policy")
		}
	}
}
