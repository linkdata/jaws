package wire

import (
	"fmt"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
)

// Message contains the elements of a message to be sent to requests.
type Message struct {
	// Dest selects recipients among active Requests: nil targets every active
	// Request, a nonzero request key targets the matching active Request, and any
	// other value is expanded into a tag or tag list. Plain strings and Jid values
	// are illegal destinations.
	Dest any
	What what.What // command to perform
	Data string    // payload: inner HTML content or a tag list
}

// String returns the Message in a form suitable for debug output.
//
// Dest is rendered with [tag.TagString], so in the default build it shows only
// its type (and, for a pointer, its address when it can be read safely) and a
// malformed Dest cannot crash the caller; build with -tags debug or -race for
// full-value rendering, which is more informative but not crash-safe.
func (msg *Message) String() string {
	return fmt.Sprintf(
		"{%s, %v, %q}",
		tag.TagString(msg.Dest),
		msg.What,
		msg.Data,
	)
}
