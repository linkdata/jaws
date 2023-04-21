package jaws

import (
	"html/template"
	"reflect"
	"testing"
)

func TestHtmlInput(t *testing.T) {
	type args struct {
		jid   string
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
				jid: "some_id",
				typ: "input_type",
				val: "initial_val",
			},
			want: `<input jid="some_id" type="input_type" value="initial_val">`,
		},
		{
			name: "HtmlInput one empty attr",
			args: args{
				jid:   "some_id2",
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []string{""},
			},
			want: `<input jid="some_id2" type="input_type2" value="initial_val2">`,
		},
		{
			name: "HtmlInput one filled attr",
			args: args{
				jid:   "some_id2",
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []string{"some_attr"},
			},
			want: `<input jid="some_id2" type="input_type2" value="initial_val2" some_attr>`,
		},
		{
			name: "HtmlInput two filled attr, one empty",
			args: args{
				jid:   "some_id2",
				typ:   "input_type2",
				val:   "initial_val2",
				attrs: []string{"some_attr1", "", "some_attr2"},
			},
			want: `<input jid="some_id2" type="input_type2" value="initial_val2" some_attr1 some_attr2>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HtmlInput(tt.args.jid, tt.args.typ, tt.args.val, tt.args.attrs...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HtmlInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHtmlInner(t *testing.T) {
	type args struct {
		jid   string
		tag   string
		typ   string
		inner string
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
				jid:   "id1",
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
			},
			want: `<tag1 jid="id1" type="typ1">inner_text</tag1>`,
		},
		{
			name: "HtmlInner singleton tag",
			args: args{
				jid:   "id1",
				tag:   "img",
				typ:   "",
				inner: "",
			},
			want: `<img jid="id1">`,
		},
		{
			name: "HtmlInner two filled attrs, one empty",
			args: args{
				jid:   "id1",
				tag:   "tag1",
				typ:   "typ1",
				inner: "inner_text",
				attrs: []string{"some_attr1", "some_attr2", ""},
			},
			want: `<tag1 jid="id1" type="typ1" some_attr1 some_attr2>inner_text</tag1>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HtmlInner(tt.args.jid, tt.args.tag, tt.args.typ, tt.args.inner, tt.args.attrs...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HtmlInner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHtmlSelect(t *testing.T) {
	type args struct {
		jid   string
		val   *NamedBoolArray
		attrs []string
	}
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			name: "HtmlSelect nil NamedBoolArray",
			args: args{
				jid:   "sel1",
				attrs: []string{},
			},
			want: "<select jid=\"sel1\">\n</select>\n",
		},
		{
			name: "HtmlSelect empty NamedBoolArray and one attr",
			args: args{
				jid:   "sel2",
				val:   NewNamedBoolArray(),
				attrs: []string{"attr1"},
			},
			want: "<select jid=\"sel2\" attr1>\n</select>\n",
		},
		{
			name: "HtmlSelect NamedBoolArray with data",
			args: args{
				jid: "sel3",
				val: func() (nba *NamedBoolArray) {
					nba = NewNamedBoolArray()
					nba.Add("one", "1")
					nba.Add("two", "2")
					nba.Add("three", "2")
					nba.Check("two")
					return
				}(),
				attrs: []string{"", "attr2"},
			},
			want: `<select jid="sel3" attr2>
<option value="one">1</option>
<option value="two" selected>2</option>
<option value="three">2</option>
</select>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HtmlSelect(tt.args.jid, tt.args.val, tt.args.attrs...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HtmlSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}
