package named

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "github.com/linkdata/jaws/core"
)

func newCoreRequest(t *testing.T) (*core.Jaws, *core.Request) {
	t.Helper()
	jw, err := core.New()
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

func (noopUI) JawsRender(*core.Element, io.Writer, []any) error { return nil }

func (noopUI) JawsUpdate(*core.Element) {}
