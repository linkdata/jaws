// Package htmlio writes the small HTML fragments used by standard JaWS widgets.
//
// Attribute values passed as ordinary strings are HTML-escaped by this package.
// HTML tag names and template.HTMLAttr fragments are trusted input and are
// written as-is; callers must not derive them from untrusted user data.
package htmlio
