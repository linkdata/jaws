// Package named provides named boolean values and collections used by select,
// option, and radio widgets.
//
// Names are browser form values and must be non-empty valid UTF-8 strings without
// U+0000 (NUL). Labels are [html/template.HTML] and are rendered as trusted HTML;
// escape user-controlled text before passing it to [NewBool] or [BoolArray.Add].
//
// [BoolArray] is the standard shared selection model for
// [github.com/linkdata/jaws/lib/ui.Select] and
// [github.com/linkdata/jaws/lib/ui.RequestWriter.RadioGroup].
package named
