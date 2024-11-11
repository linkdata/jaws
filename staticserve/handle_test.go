package staticserve

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleFS(t *testing.T) {
	mux := http.NewServeMux()
	uri, err := HandleFS(assetsFS, "assets", "subdir/test.txt", mux.Handle)
	if err != nil {
		t.Error(err)
	}
	if uri != "/subdir/test.u9cvw0b8o4xe.txt" {
		t.Error(uri)
	}
	rq := httptest.NewRequest(http.MethodGet, uri, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, rq)
	if sc := rr.Result().StatusCode; sc != http.StatusOK {
		t.Error(sc)
	}
}
