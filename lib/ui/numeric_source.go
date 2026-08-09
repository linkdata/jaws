package ui

import (
	"errors"
	"fmt"

	"github.com/linkdata/jaws/lib/tag"
)

func validateEditableNumericSource(source any) (err error) {
	var tags []any
	if tags, err = tag.TagExpand(source); err != nil {
		// Keep the specific expansion error available while also providing one
		// stable identity for every unusable editable numeric source.
		if !errors.Is(err, tag.ErrNotUsableAsTag) {
			err = errors.Join(tag.ErrNotUsableAsTag, err)
		}
		return
	}
	if len(tags) == 0 {
		err = fmt.Errorf("%w: editable numeric source expands to no tags", tag.ErrNotUsableAsTag)
	}
	return
}
