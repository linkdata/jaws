package jaws

type subscription struct {
	msgCh chan Message
	rq    *Request
}
