package jaws

import "github.com/linkdata/jaws/core/jawswire"

// SubscribeForTest subscribes rq to broadcasts with the given channel size.
//
// It is intended for use by test helpers.
func (jw *Jaws) SubscribeForTest(rq *Request, size int) chan jawswire.Message {
	return jw.subscribe(rq, size)
}

// PumpSubscriptionsForTest pushes empty subscriptions through the internal queue.
//
// It is intended for use by test helpers.
func (jw *Jaws) PumpSubscriptionsForTest() {
	for i := 0; i <= cap(jw.subCh); i++ {
		jw.subCh <- subscription{}
	}
}

// ProcessForTest runs the request processing loop with explicit channels.
//
// It is intended for use by test helpers.
func (rq *Request) ProcessForTest(broadcastMsgCh chan jawswire.Message, incomingMsgCh <-chan jawswire.WsMsg, outboundMsgCh chan<- jawswire.WsMsg) {
	rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh)
}

// RecycleForTest recycles rq back into the request pool.
//
// It is intended for use by test helpers.
func (jw *Jaws) RecycleForTest(rq *Request) {
	jw.recycle(rq)
}
