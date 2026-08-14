// Package named provides named boolean values and collections used by select,
// option and radio widgets.
//
// Labels are represented as template.HTML and are rendered as trusted HTML. When
// labels come from user-controlled text, escape them before constructing the Bool
// or BoolArray entry.
//
// Names used as browser form values must be non-empty, valid UTF-8 strings
// without U+0000 (NUL).
package named
