package jawstree

import "errors"

// ErrInvalidSelection is returned by [New] and [Tree.SetSelected] when a requested
// selection violates the tree's selection policy: more than one selected node when
// the tree is single-select (neither [MultiSelectEnabled] nor [CascadeSelectChildren]
// is set), or a selected node that is disabled or the root. Match with [errors.Is].
var ErrInvalidSelection = errors.New("jawstree: invalid selection")
