package wire

import (
	"fmt"

	"github.com/linkdata/jaws/lib/what"
)

// Message contains the elements of a message to be sent to requests.
type Message struct {
	Dest any       // destination tag, HTML ID or *jaws.Element
	What what.What // command to perform
	Data string    // payload, such as inner HTML content or a slice of tags
}

// String returns the Message in a form suitable for debug output.
func (msg *Message) String() string {
	return fmt.Sprintf("{%v, %v, %q}",
		msg.Dest,
		msg.What,
		msg.Data,
	)
}
