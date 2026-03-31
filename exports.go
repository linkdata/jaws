package jaws

import (
	"github.com/linkdata/jaws/jawsdata"
	"github.com/linkdata/jaws/jawshtml"
	"github.com/linkdata/jaws/jawstags"
)

type (
	// Tag is an alias for jawstags.Tag.
	Tag = jawstags.Tag
	// TagGetter is an alias for jawstags.TagGetter.
	TagGetter = jawstags.TagGetter
)

var (
	// ErrIllegalTagType is returned when a UI tag type is disallowed.
	ErrIllegalTagType = jawstags.ErrIllegalTagType
	// ErrNotComparable indicates a value cannot be used where comparability is required.
	ErrNotComparable = jawstags.ErrNotComparable
	// ErrNotUsableAsTag indicates a value cannot be used as a tag.
	ErrNotUsableAsTag = jawstags.ErrNotUsableAsTag
	// ErrTooManyTags indicates recursive tag expansion exceeded safety limits.
	ErrTooManyTags = jawstags.ErrTooManyTags
	// JawsKeyString returns the string to be used for the given JaWS key.
	JawsKeyString = jawsdata.JawsKeyString
	// WriteHTMLTag writes an HTML tag with JaWS attributes.
	WriteHTMLTag = jawshtml.WriteHTMLTag
)

const (
	// ISO8601 is the date format used by date input widgets (YYYY-MM-DD).
	ISO8601 = jawsdata.ISO8601
)
