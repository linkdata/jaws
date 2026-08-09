package bind

import (
	"fmt"
	"html"
	"html/template"

	"github.com/linkdata/jaws"
)

type htmlGetter struct{ v template.HTML }

func (g htmlGetter) JawsGetHTML(elem *jaws.Element) template.HTML {
	return g.v
}

func (g htmlGetter) JawsGetTag() any {
	return nil
}

type htmlStringerGetter struct{ sg fmt.Stringer }

func (g htmlStringerGetter) JawsGetHTML(elem *jaws.Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.String())) // #nosec G203
}

func (g htmlStringerGetter) JawsGetTag() any {
	return g.sg
}

type htmlBinderString struct{ Binder[string] }

func (g htmlBinderString) JawsGetHTML(elem *jaws.Element) template.HTML {
	return template.HTML(html.EscapeString(g.Binder.JawsGet(elem))) // #nosec G203
}

type htmlGetterString struct{ sg Getter[string] }

func (g htmlGetterString) JawsGetHTML(elem *jaws.Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.JawsGet(elem))) // #nosec G203
}

func (g htmlGetterString) JawsGetTag() any {
	return g.sg
}

// MakeHTMLGetter returns an [HTMLGetter] for value.
//
// The first matching conversion is used:
//
//   - [HTMLGetter] is returned unchanged.
//   - [template.HTML] is used unchanged.
//   - Binder[string] and Getter[string] use escaped [Getter.JawsGet] output.
//   - [fmt.Stringer] uses escaped [fmt.Stringer.String] output.
//   - string is used unchanged.
//   - Other values use escaped [fmt.Sprint] output.
//
// Getter[string] and [fmt.Stringer] adapters expose the wrapped value as an
// implicit tag. The value must be accepted by [tag.TagExpand], directly or
// through [tag.TagGetter]; JawsGetTag may return nil to leave it untagged.
//
// Plain strings are not escaped; do not pass untrusted text as a plain string.
func MakeHTMLGetter(value any) HTMLGetter {
	switch v := value.(type) {
	case HTMLGetter:
		return v
	case template.HTML:
		return htmlGetter{v}
	case Binder[string]:
		return htmlBinderString{v}
	case Getter[string]:
		return htmlGetterString{v}
	case fmt.Stringer:
		return htmlStringerGetter{v}
	case string:
		return htmlGetter{template.HTML(v)} // #nosec G203
	default:
		return htmlGetter{template.HTML(html.EscapeString(fmt.Sprint(v)))} // #nosec G203
	}
}
