// Package ui contains the standard JaWS widgets and template helpers.
//
// Its main building blocks are [HTMLInner] for dynamic inner HTML; [Input],
// [InputText], [InputBool], and [InputDate] for typed controls; [Number] and
// [Range] for numeric controls; [Container], [Tbody], [Select], and [MultiSelect]
// for dynamic children; and [Template], [Handler], and [RequestWriter] for templates.
//
// Widgets follow [github.com/linkdata/jaws.UI]'s request ownership and equality
// requirements. Input widgets retain state for one live Element; other widgets
// document their supported multiplicity on their concrete types.
//
// [HTMLInner] widgets adapt content with
// [github.com/linkdata/jaws/lib/bind.MakeHTMLGetter]. Plain strings are trusted
// HTML; adapters for string-valued getters escape their text. Existing
// [github.com/linkdata/jaws/lib/bind.HTMLGetter] values retain their own rendering
// behavior. Render-parameter strings are trusted raw attributes.
//
// The [widget guide] covers templates, inputs, containers, and custom widgets.
//
// [widget guide]: https://github.com/linkdata/jaws/blob/main/doc/ui/README.md
package ui
