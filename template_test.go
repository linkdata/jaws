package jaws

import (
	"html/template"
	"strings"
	"testing"
)

func TestTemplate_String(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	dot := 123
	tmpl := rq.MakeTemplate("testtemplate", dot)

	is.Equal(tmpl.String(), `{"testtemplate", 123}`)
}

func TestTemplate_Calls_Dot_Updater(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	dot := &testUi{}
	tmpl := rq.MakeTemplate("testtemplate", dot)
	tmpl.JawsUpdate(nil)
	if dot.updateCalled != 1 {
		t.Error(dot.updateCalled)
	}
}

func TestRequest_MustTemplate(t *testing.T) {
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
			if gotTp := rq.MustTemplate(tt.arg); gotTp != tt.wantTp {
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
			rq.MustTemplate(tt.arg)
			t.Fail()
		})
	}
}
