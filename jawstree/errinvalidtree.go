package jawstree

import "errors"

// ErrInvalidTree is returned by [New] when the supplied node graph cannot back a
// [Tree]: a nil root, a cyclic or shared-node graph (a node reachable more than
// once), a negative or unknown [Option] bit, or more nodes than [MaxTreeNodes].
// The error text carries the specific reason; match the class with [errors.Is].
var ErrInvalidTree = errors.New("jawstree: invalid tree")
