package named

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
)

func newCoreRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("nil request")
	}
	return jw, rq
}

type noopUI struct{}

func (noopUI) JawsRender(*jaws.Element, io.Writer, []any) error { return nil }

func (noopUI) JawsUpdate(*jaws.Element) {}
