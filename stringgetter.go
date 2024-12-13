package jaws

import (
	"fmt"
	"html/template"
)

type StringGetter interface {
	JawsGetString(e *Element) string
}

type stringGetterT struct {
	Getter[string]
}

func (g stringGetterT) JawsGetString(e *Element) string {
	return g.JawsGet(e)
}

func (g stringGetterT) JawsGetTag(rq *Request) any {
	return g.Getter
}

type stringerGetter struct{ v fmt.Stringer }

func (g stringerGetter) JawsGetString(e *Element) string {
	return g.v.String()
}

func (g stringerGetter) JawsGetTag(rq *Request) any {
	return g.v
}

type stringGetterStatic struct{ v string }

func (g stringGetterStatic) JawsGetString(e *Element) string {
	return g.v
}

func (g stringGetterStatic) JawsGetTag(rq *Request) any {
	return nil
}

func makeStringGetter(v any) StringGetter {
	switch v := v.(type) {
	case StringGetter:
		return v
	case Getter[string]:
		return stringGetterT{v}
	case fmt.Stringer:
		return stringerGetter{v}
	case string:
		return stringGetterStatic{v}
	case template.HTML:
		return stringGetterStatic{string(v)}
	case template.HTMLAttr:
		return stringGetterStatic{string(v)}
	}
	panic(fmt.Errorf("expected jaws.StringGetter, jaws.Getter[string], fmt.Stringer or string, not %T", v))
}
