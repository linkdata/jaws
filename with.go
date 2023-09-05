package jaws

type With struct {
	*Request
	Dot interface{}
}

func (rq *Request) With(dot interface{}) With {
	return With{Request: rq, Dot: dot}
}
