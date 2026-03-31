package testutil

import "github.com/linkdata/jaws/core/tags"

// SelfTagger returns itself as the tag value.
type SelfTagger struct{}

// JawsGetTag returns st itself.
func (st *SelfTagger) JawsGetTag(tags.Context) any {
	return st
}
