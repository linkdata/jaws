package staticserve_test

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

//go:embed assets
var assetsFS embed.FS

func Test_MuxServeFS(t *testing.T) {
	mux := http.NewServeMux()
	uris, err := staticserve.MuxServeFS(mux, "/", assetsFS)
	if err != nil {
		t.Error(err)
	}
	if len(uris) != 2 {
		t.Error("expected two uris")
	}
	for fn, uri := range uris {
		t.Log(fn, uri)
		rq := httptest.NewRequest(http.MethodGet, uri, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		if sc := rr.Result().StatusCode; sc != http.StatusOK {
			t.Error(sc)
		}
	}
}
