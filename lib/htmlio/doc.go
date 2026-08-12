// Package htmlio writes the small HTML fragments used by standard JaWS widgets.
//
// Generated attribute values are encoded by [AppendAttrValue]. HTML tag names
// and template.HTMLAttr fragments are trusted and written as-is; callers must
// not derive them from untrusted data. Use [Attr] or [AppendAttr] to build an
// attribute whose value is escaped.
package htmlio
