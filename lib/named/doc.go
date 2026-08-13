// Package named provides named boolean values and collections used by select,
// option and radio widgets.
//
// Labels are represented as template.HTML and are rendered as trusted HTML. When
// labels come from user-controlled text, escape them before constructing the Bool
// or BoolArray entry.
//
// Names used as browser form values must be valid UTF-8 and must not contain
// U+0000 (NUL), so their identity survives an HTML and browser round trip.
package named
