package jawstree

import (
	"encoding/json"
	"slices"
	"strconv"
	"unicode/utf8"
)

// Node is one tree node rendered by [Tree].
//
// Concurrency: once the owning [Tree] has been rendered, its Node tree is shared
// with the JaWS event goroutines, which access it under the Tree's lock. The
// exported Node accessors below are not internally synchronized, so callers must
// hold that lock when using them on a rendered Tree: the Tree's read lock (RLock)
// for the read-only helpers ([Node.Walk], [Node.HasNames], [Node.GetNames],
// [Node.GetSelected], [Node.MarshalJSON]). No locking is needed before the Tree is
// rendered (for example while building it in [New]).
//
// The struct json tags are documentation only: the wire encoding sent to Quercus.js
// is produced by [Node.MarshalJSON], and Disabled is emitted inverted (as
// "selectable":false), so the tags do not all mirror the wire shape.
type Node struct {
	Parent   *Node   `json:"-"`                 // parent node, set by New, nil for root
	Name     string  `json:"name"`              // display name; do not change after New (see [Tree])
	ID       string  `json:"id,omitzero"`       // positional JSON path, set by New; also the DOM data-id
	Selected bool    `json:"selected,omitzero"` // selected state
	Disabled bool    `json:"disabled,omitzero"` // emitted as "selectable":false (inverted) on the wire
	Children []*Node `json:"children,omitzero"` // child nodes
}

// appendJSONString appends s as a JSON string literal. Unlike
// [strconv.AppendQuote] it always produces valid JSON: invalid UTF-8 is replaced
// with U+FFFD rather than emitted as a Go-only \xNN escape (which would break
// JSON.parse on the client). Marshaling a string never returns an error.
func appendJSONString(b []byte, s string) []byte {
	enc, _ := json.Marshal(s)
	return append(b, enc...)
}

// jsonStringLen reports how many bytes s occupies as a wire JSON string literal
// (surrounding quotes plus escaping, matching [appendJSONString] and encoding/json),
// but stops once the running length exceeds limit and returns a value greater than
// limit. It never allocates, so New can weigh an untrusted [Node.Name] — and reject an
// over-long or escape-heavy one — without encoding a full copy of it.
func jsonStringLen(s string, limit int64) int64 {
	n := int64(2) // the surrounding quotes
	for i := 0; i < len(s); {
		if n > limit {
			return n
		}
		if c := s[i]; c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\' || c == '\b' || c == '\f' || c == '\n' || c == '\r' || c == '\t':
				n += 2
			case c < 0x20 || c == '<' || c == '>' || c == '&':
				n += 6 // control bytes and the HTML-escaped set become \u00xx
			default:
				n++
			}
			i++
			continue
		}
		switch r, size := utf8.DecodeRuneInString(s[i:]); {
		case r == utf8.RuneError && size == 1:
			n += 6 // invalid UTF-8 becomes the \ufffd escape
			i++
		case r == '\u2028' || r == '\u2029':
			n += 6 // the JSON line/paragraph separators are escaped
			i += size
		default:
			n += int64(size)
			i += size
		}
	}
	return n
}

func (node *Node) marshalJSON(b []byte) []byte {
	b = append(b, `{"name":`...)
	b = appendJSONString(b, node.Name)
	if node.ID != "" {
		b = append(b, `,"id":`...)
		b = appendJSONString(b, node.ID)
	}
	if node.Selected {
		b = append(b, `,"selected":true`...)
	}
	if node.Disabled {
		// Quercus.js expects "selectable"; the server tracks the inverse (Disabled).
		b = append(b, `,"selectable":false`...)
	}
	b = append(b, `,"children":[`...)
	first := true
	for _, c := range node.Children {
		if c == nil { // defensive: New strips nil children before rendering
			continue
		}
		if !first {
			b = append(b, ',')
		}
		first = false
		b = c.marshalJSON(b)
	}
	b = append(b, "]}"...)
	return b
}

// MarshalJSON writes the Quercus.js JSON shape for node.
func (node Node) MarshalJSON() (b []byte, err error) {
	// The receiver is a value, not a pointer, so json.Marshal routes both a Node and a
	// *Node here; a pointer receiver would let a non-addressable Node value fall back to
	// the struct tags and emit a different shape ("disabled":true, not "selectable":false).
	// marshalJSON is the canonical encoder; MarshalJSON just delegates to it.
	b = node.marshalJSON(nil)
	return
}

var (
	_ json.Marshaler = Node{}
	_ json.Marshaler = (*Node)(nil)
)

// Walk calls fn for node and all descendants with their JSON paths.
//
// node is visited with the supplied jsPath; callers pass "" for the root. Each
// descendant is visited with "children.<i>" appended to its parent's path, where i
// is the child's index in node.Children. A nil child is skipped while i keeps the
// raw slice index, so on a Tree built by [New] — where [Node.stripNilChildren] has
// already removed nil entries — these indices stay dense and match the wire
// positions emitted by [Node.marshalJSON].
func (node *Node) Walk(jsPath string, fn func(jsPath string, node *Node)) {
	fn(jsPath, node)
	if jsPath != "" {
		jsPath += "."
	}
	for i, child := range node.Children {
		if child == nil { // defensive: New strips nil children before rendering
			continue
		}
		child.Walk(jsPath+"children."+strconv.Itoa(i), fn)
	}
}

// HasNames reports whether node matches names as a path from the root.
//
// The root (nil Parent) matches only an empty names slice. The match walks the
// parent chain, comparing each name against the corresponding ancestor, so a call
// is O(len(names)); resolving large selections over deep trees is therefore
// O(nodes x depth x paths).
func (node *Node) HasNames(names []string) (yes bool) {
	if node.Parent == nil {
		return len(names) == 0
	}
	if len(names) == 0 {
		return false
	}
	return node.Name == names[len(names)-1] && node.Parent.HasNames(names[:len(names)-1])
}

// GetNames returns the path of names from the root to node.
func (node *Node) GetNames() (names []string) {
	for node.Parent != nil {
		names = append(names, node.Name)
		node = node.Parent
	}
	slices.Reverse(names)
	return
}

// GetSelected returns the name-paths (root-to-node name lists) of all selected nodes.
//
// Selection is reported by name-path, not by the unique node identity used on the
// wire. If sibling nodes share the same name their name-paths are identical, so the
// round-trip is lossy: [Tree.SetSelected] cannot tell them apart and selects every
// sibling sharing a selected name-path. Give siblings distinct names if they must be
// addressed independently through this API, or use the index-based wire selection.
func (node *Node) GetSelected() (nameLists [][]string) {
	node.Walk("", func(jsPath string, node *Node) {
		if node.Selected {
			nameLists = append(nameLists, node.GetNames())
		}
	})
	return
}
