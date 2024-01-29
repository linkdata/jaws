package jaws

import (
	"html/template"
	"reflect"
	"testing"
)

type testHtmlGetter string

func (g testHtmlGetter) JawsGetHtml(e *Element) template.HTML {
	return template.HTML(g)
}

var _ HtmlGetter = testHtmlGetter("foo")

func TestRequest_A(t *testing.T) {
	type args struct {
		innerHtml any
		params    []any
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "string",
			args: args{
				innerHtml: "string",
				params:    []any{},
			},
			want: `<a id="Jid.1">string</a>`,
		},
		{
			name: "template.HTML",
			args: args{
				innerHtml: template.HTML("<div></div>"),
				params:    []any{`href="#"`},
			},
			want: `<a id="Jid.1" href="#"><div></div></a>`,
		},
		{
			name: "HtmlGetter",
			args: args{
				innerHtml: testHtmlGetter("<div></div>"),
				params:    []any{},
			},
			want: `<a id="Jid.1"><div></div></a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextJid = 0
			rq := newTestRequest()
			defer rq.Close()
			rq.A(tt.args.innerHtml, tt.args.params...)
			if got := rq.BodyHtml(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Request.A() = %v, want %v", got, tt.want)
			}
		})
	}
}
