package jaws

import (
	"html/template"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Template(t *testing.T) {
	is := testHelper{t}
	type args struct {
		templ  interface{}
		dot    interface{}
		params []interface{}
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
				1234,
				[]any{"hidden"},
			},
			want:   `<div id="Jid.1" hidden>1234</div>`,
			tags:   nil,
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
			nextJid = 0
			rq := newTestRequest()
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
			rq.Template(tt.args.templ, tt.args.dot, tt.args.params...)
			got := rq.BodyHtml()
			is.Equal(len(rq.elems), 1)
			elem := rq.elems[0]
			if tt.errtxt != "" {
				t.Fail()
			}
			gotTags := elem.TagsOf(elem)
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
	is := testHelper{t}
	rq := newTestRequest()
	defer rq.Close()
	dot := &templateDot{clickedCh: make(chan struct{})}
	rq.Template("testtemplate", dot)
	rq.jw.Broadcast(Message{
		Dest: dot,
		What: what.Update,
	})
	rq.jw.Broadcast(Message{
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
