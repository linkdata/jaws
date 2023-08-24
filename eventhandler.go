package jaws

import "github.com/linkdata/jaws/what"

type EventHandler interface {
	JawsEvent(e *Element, wht what.What, val string) (err error)
}
