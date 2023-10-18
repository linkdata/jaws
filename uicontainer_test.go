package jaws

import (
	"html/template"
	"reflect"
	"testing"
)

type testContainer struct{ contents []UI }

func (tc *testContainer) JawsContains(rq *Request) (contents []UI) {
	return tc.contents
}

var _ Container = &testContainer{}

func TestRequest_Container(t *testing.T) {
	type args struct {
		c      Container
		params []interface{}
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "empty",
			args: args{
				c:      &testContainer{},
				params: []interface{}{},
			},
			want: `<div id="Jid.1"></div>`,
		},
		{
			name: "one",
			args: args{
				c:      &testContainer{[]UI{NewUiSpan(testHtmlGetter("foo"))}},
				params: []interface{}{"hidden"},
			},
			want: `<div id="Jid.1" hidden><span id="Jid.2">foo</span></div>`,
		},
		{
			name: "two",
			args: args{
				c:      &testContainer{[]UI{NewUiSpan(testHtmlGetter("foo")), NewUiSpan(testHtmlGetter("bar"))}},
				params: []interface{}{"hidden"},
			},
			want: `<div id="Jid.1" hidden><span id="Jid.2">foo</span><span id="Jid.3">bar</span></div>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextJid = 0
			rq := newTestRequest()
			defer rq.Close()
			if got := rq.Container("div", tt.args.c, tt.args.params...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Request.Container() = %v, want %v", got, tt.want)
			}
		})
	}
}
