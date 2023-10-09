package jaws

import "sync/atomic"

type UiInput struct {
	UiHtml
	Last atomic.Value
}
