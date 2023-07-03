package jaws

type subscription struct {
	msgCh chan *Message
	elems map[string]EventFn
}
