package jaws

import "github.com/linkdata/jaws/core/tags"

// TagContext is passed to TagGetter.JawsGetTag when resolving dynamic tags.
type TagContext = tags.Context

type TagGetter = tags.TagGetter
