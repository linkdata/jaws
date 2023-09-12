package jaws

import (
	"fmt"
	"html/template"
)

type Inner struct{ Value template.HTML }

func (rq *Request) Inner(param string) (vp ValueProxy) {
	return &Inner{Value: template.HTML(param)}
}

func (in *Inner) JawsGet(e *Element) interface{} {
	return in.Value
}

func (in *Inner) JawsSet(e *Element, val interface{}) bool {
	panic(fmt.Errorf("jaws: %v: requires ValueProxy or *atomic.Value instead of Inner", e))
}
