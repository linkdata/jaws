package jawstree

import "errors"

// ErrPathRejected is returned by [Node.JawsSetPath] when a client-initiated
// path write is refused: a path that does not address a per-node ".selected"
// flag, a non-bool value, or a malformed or out-of-range child index. The
// error text carries the specific reason; match the class with [errors.Is].
var ErrPathRejected = errors.New("jawstree: refusing client path-set")
