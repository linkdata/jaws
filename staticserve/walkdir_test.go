package staticserve_test

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

//go:embed assets
var assetsFS embed.FS

func Test_WalkDir(t *testing.T) {
	var uris []string
	mux := http.NewServeMux()
	err := staticserve.WalkDir(assetsFS, "assets", func(filepath string, ss *staticserve.StaticServe) (err error) {
		uri := path.Join("/static", ss.Name)
		t.Log(filepath, uri)
		uris = append(uris, uri)
		mux.Handle(uri, ss)
		return
	})
	if err != nil {
		t.Error(err)
	}
	if len(uris) != 2 {
		t.Error("expected two uris")
	}
	for _, uri := range uris {
		rq := httptest.NewRequest(http.MethodGet, uri, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		if sc := rr.Result().StatusCode; sc != http.StatusOK {
			t.Error(sc)
		}
	}
}
