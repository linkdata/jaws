package jawstree

import "errors"

// ErrInvalidSelection identifies a selection rejected by a Tree policy.
//
// [New] and [Tree.SetSelected] return it for more than one selected node in ordinary
// single-select mode, a disconnected selection in cascade-only mode, selection on a
// tree configured with [NodeSelectionDisabled], or a selected node that is disabled
// or the root. Match with [errors.Is].
var ErrInvalidSelection = errors.New("jawstree: invalid selection")
