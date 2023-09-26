package jaws

type TagGetter interface {
	JawsGetTag(e *Element) interface{}
}
