package jaws

type TagGetter interface {
	JawsGetTag(rq *Request) interface{}
}
