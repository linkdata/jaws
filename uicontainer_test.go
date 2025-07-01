package jaws

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/linkdata/jaws/what"
)

type testContainer struct{ contents []UI }

func (tc *testContainer) JawsContains(e *Element) (contents []UI) {
	return tc.contents
}

var _ Container = &testContainer{}

func TestRequest_Container(t *testing.T) {
	type args struct {
		c      Container
		params []any
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
				params: []any{},
			},
			want: `<div id="Jid.1"></div>`,
		},
		{
			name: "one",
			args: args{
				c:      &testContainer{[]UI{NewUiSpan(testHTMLGetter("foo"))}},
				params: []any{"hidden"},
			},
			want: `<div id="Jid.1" hidden><span id="Jid.2">foo</span></div>`,
		},
		{
			name: "two",
			args: args{
				c:      &testContainer{[]UI{NewUiSpan(testHTMLGetter("foo")), NewUiSpan(testHTMLGetter("bar"))}},
				params: []any{"hidden"},
			},
			want: `<div id="Jid.1" hidden><span id="Jid.2">foo</span><span id="Jid.3">bar</span></div>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextJid = 0
			rq := newTestRequest()
			defer rq.Close()
			err := rq.Container("div", tt.args.c, tt.args.params...)
			if err != nil {
				t.Error(err)
			}
			if got := rq.BodyHTML(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Request.Container()\nwant %v\n got %v", tt.want, got)
			}
		})
	}
}

func TestRequest_Container_Alteration(t *testing.T) {
	span1 := NewUiSpan(MakeHTMLGetter("span1"))
	span2 := NewUiSpan(MakeHTMLGetter("span2"))
	span3 := NewUiSpan(MakeHTMLGetter("span3"))
	tests := []struct {
		name string
		c    *testContainer
		l    []UI
		want []wsMsg
	}{
		{
			name: "no change",
			c:    &testContainer{contents: []UI{span1, span2, span3}},
			l:    []UI{span1, span2, span3},
			want: []wsMsg{},
		},
		{
			name: "add one to empty",
			c:    &testContainer{},
			l:    []UI{span1},
			want: []wsMsg{
				{
					Data: `<span id="Jid.2">span1</span>`,
					Jid:  1,
					What: what.Append,
				},
				{
					Data: `Jid.2`,
					Jid:  1,
					What: what.Order,
				},
			},
		},
		{
			name: "append two",
			c:    &testContainer{contents: []UI{span1}},
			l:    []UI{span1, span2, span3},
			want: []wsMsg{
				{
					Data: `<span id="Jid.3">span2</span>`,
					Jid:  1,
					What: what.Append,
				},
				{
					Data: `<span id="Jid.4">span3</span>`,
					Jid:  1,
					What: what.Append,
				},
				{
					Data: `Jid.2 Jid.3 Jid.4`,
					Jid:  1,
					What: what.Order,
				},
			},
		},
		{
			name: "remove first",
			c:    &testContainer{contents: []UI{span1, span2, span3}},
			l:    []UI{span2, span3},
			want: []wsMsg{
				{
					Data: `Jid.2`,
					Jid:  1,
					What: what.Remove,
				},
				{
					Data: `Jid.3 Jid.4`,
					Jid:  1,
					What: what.Order,
				},
			},
		},
		{
			name: "reorder and replace",
			c:    &testContainer{contents: []UI{span1, span2}},
			l:    []UI{span3, span1},
			want: []wsMsg{
				{
					Data: `Jid.3`,
					Jid:  1,
					What: what.Remove,
				},
				{
					Data: `<span id="Jid.4">span3</span>`,
					Jid:  1,
					What: what.Append,
				},
				{
					Data: `Jid.4 Jid.2`,
					Jid:  1,
					What: what.Order,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, _ := New()
			defer jw.Close()
			nextJid = 0
			rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
			ui := NewUiContainer("div", tt.c)
			elem := rq.NewElement(ui)
			var sb strings.Builder
			if err := ui.JawsRender(elem, &sb, nil); err != nil {
				t.Fatal(err)
			}
			tt.c.contents = tt.l
			elem.JawsUpdate()
			if !slices.Equal(rq.wsQueue, tt.want) {
				t.Errorf("got %v, want %v", rq.wsQueue, tt.want)
			}
		})
	}
}
