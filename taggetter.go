package jaws

type TagGetter interface {
	JawsGetTag(rq *Request) interface{} // Note that the Request may be nil
}
