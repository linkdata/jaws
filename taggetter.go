package jaws

type TagGetter interface {
	JawsGetTag(rq RequestIf) any // Note that the Request may be nil
}
