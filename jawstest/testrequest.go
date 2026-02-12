package jawstest

import (
	"net/http"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/ui"
)

// TestRequest wraps jaws.TestRequest with ui.RequestWriter helpers.
type TestRequest struct {
	*jaws.TestRequest
	rw ui.RequestWriter
}

// NewTestRequest forwards to jaws.NewTestRequest.
func NewTestRequest(jw *jaws.Jaws, hr *http.Request) *TestRequest {
	tr := jaws.NewTestRequest(jw, hr)
	return &TestRequest{
		TestRequest: tr,
		rw: ui.RequestWriter{
			Request: tr.Request,
			Writer:  tr.ResponseRecorder,
		},
	}
}

func (tr *TestRequest) UI(widget jaws.UI, params ...any) error {
	return tr.rw.UI(widget, params...)
}

func (tr *TestRequest) Template(name string, dot any, params ...any) error {
	return tr.rw.Template(name, dot, params...)
}

func (tr *TestRequest) JsVar(name string, jsvar any, params ...any) error {
	return tr.rw.JsVar(name, jsvar, params...)
}

func (tr *TestRequest) Register(updater jaws.Updater, params ...any) jaws.Jid {
	return tr.rw.Register(updater, params...)
}
