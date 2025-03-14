package jaws

import (
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/jid"
)

func Test_WriteHTMLInner(t *testing.T) {
	type args struct {
		jid   jid.Jid
		tag   string
		typ   string
		inner template.HTML
		attrs []template.HTMLAttr
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "HTMLInner no attrs",
			args: args{
				jid:   1,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
			},
			want: `<tag1 id="Jid.1" type="typ1">inner_text</tag1>`,
		},
		{
			name: "HTMLInner singleton tag",
			args: args{
				jid:   2,
				tag:   "img",
				typ:   "",
				inner: "",
			},
			want: `<img id="Jid.2">`,
		},
		{
			name: "HTMLInner two filled attrs, one empty",
			args: args{
				jid:   3,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
				attrs: []template.HTMLAttr{"some_attr1", "some_attr2", ""},
			},
			want: `<tag1 id="Jid.3" type="typ1" some_attr1 some_attr2>inner_text</tag1>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := WriteHTMLInner(&sb, tt.args.jid, tt.args.tag, tt.args.typ, tt.args.inner, tt.args.attrs...); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("HTMLInner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_WriteHTMLSelect(t *testing.T) {
	type args struct {
		jid   jid.Jid
		val   *NamedBoolArray
		attrs []template.HTMLAttr
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "HTMLSelect empty NamedBoolArray and one attr",
			args: args{
				jid:   2,
				val:   NewNamedBoolArray(),
				attrs: []template.HTMLAttr{"attr1"},
			},
			want: "<select id=\"Jid.2\" attr1>\n</select>",
		},
		{
			name: "HTMLSelect NamedBoolArray with data",
			args: args{
				jid: 3,
				val: func() (nba *NamedBoolArray) {
					nba = NewNamedBoolArray()
					nba.Add("one", "1")
					nba.Add("two", "2")
					nba.Add("three", "2")
					nba.Set("two", true)
					return
				}(),
				attrs: []template.HTMLAttr{"", "attr2"},
			},
			want: `<select id="Jid.3" attr2>
<option value="one">1</option>
<option value="two" selected>2</option>
<option value="three">2</option>
</select>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := WriteHTMLSelect(&sb, tt.args.jid, tt.args.val, tt.args.attrs); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("HTMLSelect():\n got = %q\nwant = %q", got, tt.want)
			}
		})
	}
}

func Test_WriteHTMLInput(t *testing.T) {
	type args struct {
		jid   jid.Jid
		typ   string
		val   string
		attrs []template.HTMLAttr
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "HTMLInput no attrs",
			args: args{
				jid: 1,
				typ: "input_type",
				val: "initial_val",
			},
			want: `<input id="Jid.1" type="input_type" value="initial_val">`,
		},
		{
			name: "HTMLInput one empty attr",
			args: args{
				jid:   2,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []template.HTMLAttr{""},
			},
			want: `<input id="Jid.2" type="input_type2" value="initial_val2">`,
		},
		{
			name: "HTMLInput one filled attr",
			args: args{
				jid:   3,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []template.HTMLAttr{"some_attr"},
			},
			want: `<input id="Jid.3" type="input_type2" value="initial_val2" some_attr>`,
		},
		{
			name: "HTMLInput two filled attr, one empty",
			args: args{
				jid:   4,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []template.HTMLAttr{"some_attr1", "", "some_attr2"},
			},
			want: `<input id="Jid.4" type="input_type2" value="initial_val2" some_attr1 some_attr2>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			if err := WriteHTMLInput(&sb, tt.args.jid, tt.args.typ, tt.args.val, tt.args.attrs); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("HTMLInput() = %v, want %v", got, tt.want)
			}
		})
	}
}
