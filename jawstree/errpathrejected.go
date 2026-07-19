package jawstree

import "errors"

// ErrPathRejected identifies structurally invalid browser selection input.
//
// [Tree.JawsInput] uses it for a malformed payload, an out-of-range or root node
// index, a disabled node, or an absolute selection that the configured mode cannot
// represent. Other selection-policy failures may match [ErrInvalidSelection]
// instead. The error text carries the specific reason; match the class with
// [errors.Is].
var ErrPathRejected = errors.New("jawstree: refusing client selection write")
