package core

type TagGetter interface {
	JawsGetTag(rq *Request) any // Note that the Request may be nil
}
