package named

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
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

// newTestRequest creates a running test request with its Jaws Serve loop and
// request process loop started here. Those goroutines are bubbled, so this must
// be called from within synctest.Test, and the caller must defer
// closeBubbleRequest to shut them down before the bubble ends.
func newTestRequest(t *testing.T) (*jaws.Jaws, *jawstest.TestRequest) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	rq := jawstest.NewTestRequest(jw, nil)
	if rq == nil {
		jw.Close()
		t.Fatal("nil test request")
	}
	<-rq.ReadyCh
	return jw, rq
}

// closeBubbleRequest shuts the test request and its Jaws down from within a
// synctest bubble, then waits for the bubbled goroutines to exit so
// synctest.Test sees no leaked goroutines.
func closeBubbleRequest(jw *jaws.Jaws, rq *jawstest.TestRequest) {
	rq.Close()
	jw.Close()
	synctest.Wait()
}

type noopUI struct{}

func (noopUI) JawsRender(elem *jaws.Element, w io.Writer, params []any) error { return nil }

func (noopUI) JawsUpdate(elem *jaws.Element) {}
