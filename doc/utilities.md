# HTML, choices, and template loading

[Documentation](README.md) · [UI](ui/README.md) · [Bindings](bindings.md) · [Tags](tags.md)

These packages support custom widgets and application setup. Standard widgets
already use them; most pages can work through the [UI API](ui/README.md).

## Write HTML

`htmlio` writes HTML fragments for low-level widgets. Tag names, attribute names,
`template.HTML` contents, and `template.HTMLAttr` fragments are trusted inputs
written verbatim. Attribute values are escaped by `Attr`, `AppendAttr`, and
`AppendAttrValue`.

Escape user-controlled text before converting it to `template.HTML`, and pass
the unescaped logical value to `Attr`:

```go
safeText := template.HTML(template.HTMLEscapeString(userText))
err := htmlio.WriteHTMLInner(w, elem.Jid(), "span", "", safeText,
    htmlio.Attr("title", userText))
```

Return the writer error from a widget's `JawsRender`. Use application-controlled
tag and attribute names, even when their values are escaped. See the
[runnable HTML example](../lib/htmlio/example_test.go) and
[HTML helper API](../lib/htmlio/writehtml.go).

`WriteHTMLTag` writes a start tag and optional ID, type, value, and attribute
fragments. `WriteHTMLInput` is its input wrapper. `WriteHTMLInner` adds trusted
content and a closing tag for non-void elements; void elements ignore content.
It does not add a value attribute, so pass `htmlio.Attr("value", value)` when
needed. IDs are emitted only for positive Jids.

Attribute values preserve carriage returns as `&#13;` and replace U+0000 with
U+FFFD. Inner HTML keeps carriage returns verbatim; textarea values may normalize
them in the browser. Leading LF content is preserved in `textarea` and `pre`.

## Named choices

`named.Bool` holds one named boolean with a trusted HTML label. Construct it
with `named.NewBool`; its zero value has no usable name. Names must be non-empty
valid UTF-8 without U+0000. Escape user-controlled labels before passing them to
`NewBool` or `BoolArray.Add`.

`named.BoolArray` shares a selection across options. Its zero value is an empty
single-select collection. Use distinct names for rendered choices:

```go
choices := named.NewBoolArray(false).
    Add("red", "Red").
    Add("green", "Green")
choices.Set("green", true)
```

Pass a single-select array to `ui.Select` or `RequestWriter.RadioGroup`. Use
`named.NewBoolArray(true)` for `ui.MultiSelect`. Separate `ui.Radio` bindings
remain independent Go booleans even when the browser groups their controls by
name; use `RadioGroup` for shared server-side selection.

`Bool.JawsSet`, `BoolArray.JawsSet`, and `BoolArray.JawsSetValues` dirty affected
dependency tags through the supplied Element. The plain `Set` methods do not
dirty tags.
`Bool.Set` changes only that boolean; `BoolArray.Set` also applies its
single-select policy. After a plain setter changes live application state,
dirty the affected [dependency tags](tags.md).

`BoolArray.JawsGetValues` returns all checked names in array order.
`JawsSetValues` replaces the whole selection, ignores unknown names, and clears
absent names. Nil or empty input clears all choices. On a single-select array,
only the first matching name in array order is selected. An unchanged selection
returns `jaws.ErrValueUnchanged`.

`ReadLocked` and `WriteLocked` support custom collection access. Their callbacks
must not call methods that reacquire the array lock. A `Bool` keeps its original
array association after removal; remove its live UI and allow in-flight events
to finish before removing it from the collection. Reinsert it before calling
its `JawsSet`. See the full [collection contracts](../lib/named/namedboolarray.go)
and [selection examples](../lib/named/example_test.go).

## Reload templates during development

`templatereloader.New` returns a `jaws.TemplateLookuper`. Normal builds parse
templates once from an `fs.FS`. Debug and race builds parse the corresponding
disk files and reload them as they change.

```go
//go:embed assets
var templates embed.FS

lookuper, err := templatereloader.New(templates, "assets/*.html", ".")
if err != nil {
    return err
}
if err := jw.AddTemplateLookuper(lookuper); err != nil {
    return err
}
```

The glob must work with both `io/fs.Glob` and `filepath.Glob`. The disk root
passed as the third argument must mirror the embedded filesystem. It is unused
in normal builds. A directory embed omits names beginning with `.` or `_`; use
`//go:embed all:assets` if these files must be included.

In reload mode, `Lookup` checks for changes by reparsing at most once per second.
A parse failure keeps the last successful templates and waits for the next
interval before retrying. Initial parse failure returns a nil lookuper and an
error. A zero `TemplateReloader` has no templates and returns nil from `Lookup`.

Use a type assertion to `*templatereloader.TemplateReloader` to access `Path`
and `LastError` in reload mode. Normal builds return a `*template.Template`.
See [template loading](../lib/templatereloader/templatereloader.go) and the
[Bootstrap setup](bootstrap.md) for integration.
