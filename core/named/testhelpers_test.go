package named

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "github.com/linkdata/jaws/core"
)

type testUI struct{}

func (testUI) JawsRender(*core.Element, io.Writer, []any) error { return nil }
func (testUI) JawsUpdate(*core.Element)                         {}

func newTestRequest(t *testing.T) *core.Request {
	t.Helper()
	jw, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	return jw.NewRequest(hr)
}
