package bind

import (
	"html"
	"html/template"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

type testStringer struct{}

func (testStringer) String() string {
	return "<x>"
}

type testBinderStringNoHTML struct {
	Binder[string]
}

/*type testAnySetter struct {
	Value any
}

func (ag *testAnySetter) JawsGetAny(*Element) any {
	return ag.Value
}

func (ag *testAnySetter) JawsSetAny(elem *Element, value any) error {
	ag.Value = value
	return nil
}*/

func Test_MakeHTMLGetter(t *testing.T) {
	untypedText := "<span>"
	typedText := template.HTML(untypedText)
	var avUntyped, avTyped atomic.Value
	avUntyped.Store(untypedText)
	avTyped.Store(typedText)
	stringer := testStringer{}
	var binderVal = "<b>"
	var binderMu sync.Mutex
	binderNoHTML := testBinderStringNoHTML{New(&binderMu, &binderVal)}

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
		/*{
			name: "Getter[any]",
			v:    getterAny,
			want: htmlGetterAny{getterAny},
			out:  escapedTypedText,
			wantTag:  getterAny,
		},*/
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
			if gotTag := got.(tag.TagGetter).JawsGetTag(nil); gotTag != tt.wantTag {
				t.Errorf("MakeHTMLGetter(%s).JawsGetTag() = %v, want %v", tt.name, gotTag, tt.wantTag)
			}
		})
	}
}
