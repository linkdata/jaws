package jaws

import "html/template"

type Inner struct{ Value template.HTML }

func (rq *Request) Inner(param string) (in Inner) {
	return Inner{Value: template.HTML(param)}
}

func (in Inner) JawsGet(e *Element) interface{} {
	return in.Value
}
