package jaws

type Tagger interface {
	JawsTags(rq *Request, inTags []interface{}) (outTags []interface{})
}
