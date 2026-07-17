package jawstree

import "errors"

// ErrPathRejected is returned when a browser-initiated selection write is refused
// by [Tree.JawsInput]: a malformed payload, an out-of-range or root node index,
// a disabled node, or a single-select bitmap carrying more than one bit. The error
// text carries the specific reason; match the class with [errors.Is].
var ErrPathRejected = errors.New("jawstree: refusing client selection write")
