// Package htmlio writes the small HTML fragments used by standard JaWS widgets.
//
// HTML tag and attribute names, [html/template.HTML] content, and
// [html/template.HTMLAttr] fragments are trusted and are not escaped. They must
// be valid HTML and must not be derived from untrusted data. [Attr], [AppendAttr],
// and [AppendAttrValue] escape attribute values; they do not validate or escape
// names.
package htmlio
