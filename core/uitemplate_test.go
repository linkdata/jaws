package jaws

import (
	"bytes"
	"html/template"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

func TestRequest_TemplateMissingJid(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug tag not set")
	}
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("badtesttemplate").Parse(`{{with $.Dot}}<div {{$.Attrs}}>{{.}}</div>{{end}}`)))
	if e := rq.Template("badtesttemplate", nil, nil); e != nil {
		t.Error(e)
	}
	if !strings.Contains(log.String(), "WARN") || !strings.Contains(log.String(), "badtesttemplate") {
		t.Error("expected WARN in the log")
		t.Log(log.String())
	}
}

func TestRequest_TemplateJidInsideIf(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug tag not set")
	}
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("iftesttemplate").Parse(`{{with $.Dot}}{{if true}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}{{end}}`)))
	if e := rq.Template("iftesttemplate", nil, nil); e != nil {
		t.Error(e)
	}
	if strings.Contains(log.String(), "WARN") && strings.Contains(log.String(), "iftesttemplate") {
		t.Error("found WARN in the log")
		t.Log(log.String())
	}
}

func TestRequest_TemplateMissingJidButHasHTMLTag(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug tag not set")
	}
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("badtesttemplate").Parse(`<html>{{with $.Dot}}<div {{$.Attrs}}>{{.}}</div>{{end}}</html>`)))
	if e := rq.Template("badtesttemplate", nil, nil); e != nil {
		t.Error(e)
	}
	if strings.Contains(log.String(), "WARN") {
		t.Error("expected no WARN in the log")
		t.Log(log.String())
	}
}

func TestRequest_Template(t *testing.T) {
	is := newTestHelper(t)

	type intTag int

	type args struct {
		templ  string
		dot    any
		params []any
	}
	tests := []struct {
		name   string
		args   args
		want   template.HTML
		tags   []any
		errtxt string
	}{
		{
			name: "testtemplate",
			args: args{
				"testtemplate",
				intTag(1234),
				[]any{"hidden"},
			},
			want:   `<div id="Jid.1" hidden>1234</div>`,
			tags:   []any{intTag(1234)},
			errtxt: "",
		},
		{
			name: "testtemplate-with-tags",
			args: args{
				"testtemplate",
				Tag("stringtag1"),
				[]any{`style="display: none"`, Tag("stringtag2"), "hidden"},
			},
			want:   `<div id="Jid.1" style="display: none" hidden>stringtag1</div>`,
			tags:   []any{Tag("stringtag1"), Tag("stringtag2")},
			errtxt: "",
		},
	}
	// `{{with $.Dot}}<div id="{{$.Jid}}{{$.Attrs}}">{{.}}</div>{{end}}`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NextJid = 0
			rq := newTestRequest(t)
			defer rq.Close()
			if tt.errtxt != "" {
				defer func() {
					x := recover()
					if e, ok := x.(error); ok {
						if strings.Contains(e.Error(), tt.errtxt) {
							return
						}
					}
					t.Fail()
				}()
			}
			if e := rq.Template(tt.args.templ, tt.args.dot, tt.args.params...); e != nil {
				t.Error(e)
			}
			got := rq.BodyHTML()
			is.Equal(len(rq.elems), 1)
			elem := rq.elems[0]
			if tt.errtxt != "" {
				t.Fail()
			}
			gotTags := elem.Request.TagsOf(elem)
			is.Equal(len(tt.tags), len(gotTags))
			for _, tag := range tt.tags {
				is.True(elem.HasTag(tag))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Request.Template() = %v, want %v", got, tt.want)
			}
		})
	}
}

type templateDot struct {
	clickedCh chan struct{}
	gotName   string
}

func (td *templateDot) JawsClick(e *Element, name string) error {
	defer close(td.clickedCh)
	td.gotName = name
	return nil
}

var _ ClickHandler = &templateDot{}

func TestRequest_Template_Event(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	dot := &templateDot{clickedCh: make(chan struct{})}
	rq.Template("testtemplate", dot)
	rq.Jaws.Broadcast(Message{
		Dest: dot,
		What: what.Update,
	})
	rq.Jaws.Broadcast(Message{
		Dest: dot,
		What: what.Click,
		Data: "foo",
	})
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-dot.clickedCh:
	}
	is.Equal(dot.gotName, "foo")
}
