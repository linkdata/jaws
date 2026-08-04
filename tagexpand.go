package jaws

import "github.com/linkdata/jaws/lib/tag"

// MustTagExpand expands tagValue and reports expansion errors through [Jaws.MustLog].
//
// With a [Jaws.Logger] configured, the error is logged and MustTagExpand returns
// [github.com/linkdata/jaws/lib/tag.TagExpand]'s partial result. Without one,
// [Jaws.MustLog] panics, so the partial result never reaches the caller.
func (jw *Jaws) MustTagExpand(tagValue any) (result []any) {
	result, err := tag.TagExpand(tagValue)
	jw.MustLog(err)
	return
}
