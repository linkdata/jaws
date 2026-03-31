package jaws

import "github.com/linkdata/jaws/core/wire"

type subscription struct {
	msgCh chan wire.Message
	rq    *Request
}
