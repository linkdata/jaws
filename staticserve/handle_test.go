package staticserve

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleFS(t *testing.T) {
	mux := http.NewServeMux()
	uris, err := HandleFS(assetsFS, mux.Handle, "assets", "subdir/test.txt", "test.js")
	if err != nil {
		t.Error(err)
	}
	if len(uris) != 2 {
		t.Fatal(len(uris))
	}
	if uris[0] != "/subdir/test.u9cvw0b8o4xe.txt" {
		t.Error(uris[0])
	}
	if uris[1] != "/test.16sl4jy6fnyn9.js" {
		t.Error(uris[1])
	}
	rq := httptest.NewRequest(http.MethodGet, uris[0], nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, rq)
	if sc := rr.Result().StatusCode; sc != http.StatusOK {
		t.Error(sc)
	}
}
