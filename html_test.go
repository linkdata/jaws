package jaws

import (
	"html/template"
	"testing"

	"github.com/linkdata/jaws/jid"
)

func TestHtmlInput(t *testing.T) {
	type args struct {
		jid   jid.Jid
		typ   string
		val   string
		attrs []string
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "HtmlInput no attrs",
			args: args{
				jid: 1,
				typ: "input_type",
				val: "initial_val",
			},
			want: `<input id="Jid.1" type="input_type" value="initial_val">`,
		},
		{
			name: "HtmlInput one empty attr",
			args: args{
				jid:   2,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []string{""},
			},
			want: `<input id="Jid.2" type="input_type2" value="initial_val2">`,
		},
		{
			name: "HtmlInput one filled attr",
			args: args{
				jid:   3,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []string{"some_attr"},
			},
			want: `<input id="Jid.3" type="input_type2" value="initial_val2" some_attr>`,
		},
		{
			name: "HtmlInput two filled attr, one empty",
			args: args{
				jid:   4,
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []string{"some_attr1", "", "some_attr2"},
			},
			want: `<input id="Jid.4" type="input_type2" value="initial_val2" some_attr1 some_attr2>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HtmlInput(tt.args.jid, tt.args.typ, tt.args.val, tt.args.attrs...); got != tt.want {
				t.Errorf("HtmlInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHtmlInner(t *testing.T) {
	type args struct {
		jid   jid.Jid
		tag   string
		typ   string
		inner template.HTML
		attrs []string
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "HtmlInner no attrs",
			args: args{
				jid:   1,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
			},
			want: `<tag1 id="Jid.1" type="typ1">inner_text</tag1>`,
		},
		{
			name: "HtmlInner singleton tag",
			args: args{
				jid:   2,
				tag:   "img",
				typ:   "",
				inner: "",
			},
			want: `<img id="Jid.2">`,
		},
		{
			name: "HtmlInner two filled attrs, one empty",
			args: args{
				jid:   3,
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
				attrs: []string{"some_attr1", "some_attr2", ""},
			},
			want: `<tag1 id="Jid.3" type="typ1" some_attr1 some_attr2>inner_text</tag1>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HtmlInner(tt.args.jid, tt.args.tag, tt.args.typ, tt.args.inner, tt.args.attrs...); got != tt.want {
				t.Errorf("HtmlInner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHtmlSelect(t *testing.T) {
	type args struct {
		jid   jid.Jid
		val   *NamedBoolArray
		attrs []string
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "HtmlSelect empty NamedBoolArray and one attr",
			args: args{
				jid:   2,
				val:   NewNamedBoolArray(),
				attrs: []string{"attr1"},
			},
			want: "<select id=\"Jid.2\" attr1>\n</select>\n",
		},
		{
			name: "HtmlSelect NamedBoolArray with data",
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
				attrs: []string{"", "attr2"},
			},
			want: `<select id="Jid.3" attr2>
<option value="one">1</option>
<option value="two" selected>2</option>
<option value="three">2</option>
</select>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HtmlSelect(tt.args.jid, tt.args.val, tt.args.attrs...); got != tt.want {
				t.Errorf("HtmlSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}
