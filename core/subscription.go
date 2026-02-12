package core

type subscription struct {
	msgCh chan Message
	rq    *Request
}
