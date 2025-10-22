package jaws

import (
	"html/template"
	"reflect"
	"testing"
)

type testHTMLGetter string

func (g testHTMLGetter) JawsGetHTML(e *Element) template.HTML {
	return template.HTML(g)
}

var _ HTMLGetter = testHTMLGetter("foo")

func TestRequest_A(t *testing.T) {
	type args struct {
		innerHTML any
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
				innerHTML: "string",
				params:    []any{},
			},
			want: `<a id="Jid.1">string</a>`,
		},
		{
			name: "template.HTML",
			args: args{
				innerHTML: template.HTML("<div></div>"),
				params:    []any{`href="#"`},
			},
			want: `<a id="Jid.1" href="#"><div></div></a>`,
		},
		{
			name: "HTMLGetter",
			args: args{
				innerHTML: testHTMLGetter("<div></div>"),
				params:    []any{},
			},
			want: `<a id="Jid.1"><div></div></a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextJid = 0
			rq := newTestRequest(t)
			defer rq.Close()
			rq.A(tt.args.innerHTML, tt.args.params...)
			if got := rq.BodyHTML(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Request.A() = %v, want %v", got, tt.want)
			}
		})
	}
}
