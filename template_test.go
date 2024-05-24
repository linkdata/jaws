package jaws

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestTemplate_String(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	dot := 123
	tmpl := rq.Jaws.NewTemplate("testtemplate", dot)

	is.Equal(tmpl.String(), `{"testtemplate", 123}`)
}

func TestTemplate_Calls_Dot_Updater(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	dot := &testUi{}
	tmpl := rq.Jaws.NewTemplate("testtemplate", dot)
	tmpl.JawsUpdate(nil)
	if dot.updateCalled != 1 {
		t.Error(dot.updateCalled)
	}
}

func TestTemplate_As_Handler(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	dot := 123
	tmpl := rq.Jaws.NewTemplate("testtemplate", dot)
	var buf bytes.Buffer
	var rr httptest.ResponseRecorder
	rr.Body = &buf
	r := httptest.NewRequest("GET", "/", nil)
	tmpl.ServeHTTP(&rr, r)
	if got := buf.String(); got != `<div id="Jid.1" >123</div>` {
		t.Error(got)
	}
}

/*func TestRequest_MustTemplate(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	tests := []struct {
		name   string
		arg    any
		wantTp *template.Template
	}{
		{"*template.Template", rq.jw.testtmpl, rq.jw.testtmpl},
		{"named template", "testtemplate", rq.jw.testtmpl},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotTp := rq.Jaws.MustTemplate(tt.arg); gotTp != tt.wantTp {
				t.Errorf("Request.MustTemplate() = %v, want %v", gotTp, tt.wantTp)
			}
		})
	}
}

func TestRequest_MustTemplate_Panics(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	tests := []struct {
		name   string
		arg    any
		wantTp *template.Template
	}{
		{"nil", nil, rq.jw.testtmpl},
		{"nosuchtemplate", "nosuchtemplate", rq.jw.testtmpl},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				x := recover()
				if e, ok := x.(error); ok {
					if strings.Contains(e.Error(), tt.name) {
						return
					}
				}
				t.Fail()
			}()
			rq.Jaws.MustTemplate(tt.arg)
			t.Fail()
		})
	}
}*/
