package jaws

type RequestContextKey struct{}

type RequestContextValue struct {
	rq             *Request
	broadcastMsgCh chan Message
	incomingMsgCh  chan wsMsg
	outboundMsgCh  chan wsMsg
}

func (rcv *RequestContextValue) Request() *Request {
	return rcv.rq
}
