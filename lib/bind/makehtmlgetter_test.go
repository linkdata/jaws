package bind

import (
	"html"
	"html/template"
	"reflect"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

type testStringer struct{}

func (testStringer) String() string {
	return "<x>"
}

type testBinderStringNoHTML struct {
	Binder[string]
}

type testOptionalHooks struct{}

func (testOptionalHooks) JawsClick(*jaws.Element, jaws.Click) error { return nil }

func (testOptionalHooks) JawsContextMenu(*jaws.Element, jaws.Click) error { return nil }

func (testOptionalHooks) JawsInitialHTMLAttr(*jaws.Element) template.HTMLAttr { return "disabled" }

type testHookedGetter struct{ testOptionalHooks }

func (testHookedGetter) JawsGet(*jaws.Element) string { return "text" }

type testHookedStringer struct{ testOptionalHooks }

func (testHookedStringer) String() string { return "text" }

type testHookedHTMLGetter struct{ testOptionalHooks }

func (testHookedHTMLGetter) JawsGetHTML(*jaws.Element) template.HTML { return "text" }

type testHookedNamedString string

func (testHookedNamedString) JawsClick(*jaws.Element, jaws.Click) error { return nil }

func (testHookedNamedString) JawsContextMenu(*jaws.Element, jaws.Click) error { return nil }

func (testHookedNamedString) JawsInitialHTMLAttr(*jaws.Element) template.HTMLAttr { return "disabled" }

type testBinderStringMask struct {
	Binder[string]
}

func (testBinderStringMask) JawsGet(*jaws.Element) string { return "***" }

func TestMakeHTMLGetterBinderWrapperUsesJawsGet(t *testing.T) {
	value := "secret"
	var mu sync.Mutex
	masked := testBinderStringMask{New(&mu, &value).
		GetHTML(func(Binder[string], *jaws.Element) template.HTML { return "<em>secret</em>" })}
	if got := MakeHTMLGetter(masked).JawsGetHTML(nil); got != "***" {
		t.Fatalf("JawsGetHTML() = %q, want wrapper JawsGet output", got)
	}
}

func Test_MakeHTMLGetter(t *testing.T) {
	untypedText := "<span>"
	typedText := template.HTML(untypedText)
	stringer := testStringer{}
	binderVal := "<b>"
	var binderMu sync.Mutex
	binderNoHTML := testBinderStringNoHTML{New(&binderMu, &binderVal).Format("[%s]")}

	getterString := testGetterString{}

	tests := []struct {
		name    string
		v       any
		want    HTMLGetter
		out     template.HTML
		wantTag any
	}{
		{
			name:    "HTMLGetter",
			v:       htmlGetter{typedText},
			want:    htmlGetter{typedText},
			out:     typedText,
			wantTag: nil,
		},
		{
			name:    "Getter[string]",
			v:       getterString,
			want:    htmlGetterString{getterString},
			out:     template.HTML(html.EscapeString(getterString.JawsGet(nil))),
			wantTag: getterString,
		},
		{
			name:    "Binder[string]",
			v:       binderNoHTML,
			want:    htmlBinderString{binderNoHTML},
			out:     template.HTML(html.EscapeString(binderNoHTML.JawsGet(nil))),
			wantTag: &binderVal,
		},
		{
			name:    "fmt.Stringer",
			v:       stringer,
			want:    htmlStringerGetter{stringer},
			out:     template.HTML(html.EscapeString(stringer.String())),
			wantTag: stringer,
		},
		{
			name:    "template.HTML",
			v:       typedText,
			want:    htmlGetter{typedText},
			out:     typedText,
			wantTag: nil,
		},
		{
			name:    "string",
			v:       untypedText,
			want:    htmlGetter{template.HTML(untypedText)},
			out:     template.HTML(untypedText),
			wantTag: nil,
		},
		{
			name:    "int",
			v:       123,
			want:    htmlGetter{template.HTML("123")},
			out:     template.HTML("123"),
			wantTag: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeHTMLGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeHTMLGetter(%s)\n  got %#v\n want %#v", tt.name, got, tt.want)
			}
			if txt := got.JawsGetHTML(nil); txt != tt.out {
				t.Errorf("MakeHTMLGetter(%s).JawsGetHTML() = %v, want %v", tt.name, txt, tt.out)
			}
			if gotTag := got.(tag.TagGetter).JawsGetTag(); gotTag != tt.wantTag {
				t.Errorf("MakeHTMLGetter(%s).JawsGetTag() = %v, want %v", tt.name, gotTag, tt.wantTag)
			}
		})
	}
}

func TestMakeHTMLGetterOptionalHooks(t *testing.T) {
	var mu sync.Mutex
	value := "text"
	binder := New(&mu, &value)
	for _, tt := range []struct {
		name  string
		value any
		want  bool
	}{
		{"HTMLGetter", testHookedHTMLGetter{}, true},
		{"bind.New binder", binder, true},
		{"Binder interface wrapper", testBinderStringNoHTML{binder}, true},
		{"Getter", testHookedGetter{}, false},
		{"Stringer", testHookedStringer{}, false},
		{"formatted value", testHookedNamedString("text"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeHTMLGetter(tt.value)
			if _, ok := got.(jaws.ClickHandler); ok != tt.want {
				t.Errorf("ClickHandler = %t, want %t", ok, tt.want)
			}
			if _, ok := got.(jaws.ContextMenuHandler); ok != tt.want {
				t.Errorf("ContextMenuHandler = %t, want %t", ok, tt.want)
			}
			if _, ok := got.(jaws.InitialHTMLAttrHandler); ok != tt.want {
				t.Errorf("InitialHTMLAttrHandler = %t, want %t", ok, tt.want)
			}
		})
	}
}
