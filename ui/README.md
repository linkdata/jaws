# `github.com/linkdata/jaws/ui`

This package is the home of JaWS widget implementations.

## Goals

- Keep widget logic out of JaWS core request/session internals.
- Make new widget authoring local to this package.
- Provide short widget naming (`ui.Span`, `ui.NewSpan`).
- Expose template context types (`ui.RequestWriter`, `ui.With`).

## Migration

### Type and constructor names

Every legacy `jaws.UiX` / `jaws.NewUiX` maps directly to `ui.X` / `ui.NewX`.

Examples:

- `jaws.UiA` -> `ui.A`
- `jaws.NewUiA(...)` -> `ui.NewA(...)`
- `jaws.UiSpan` -> `ui.Span`
- `jaws.NewUiSpan(...)` -> `ui.NewSpan(...)`
- `jaws.UiSelect` -> `ui.Select`
- `jaws.NewUiSelect(...)` -> `ui.NewSelect(...)`

### RequestWriter helper calls

`jaws.RequestWriter` still exposes helper methods like `rw.Span(...)`,
`rw.Text(...)`, and `rw.Select(...)` for concise template use.

You can also use explicit constructors through:

```go
rw.UI(ui.NewX(...), params...)
```

Examples:

```go
rw.UI(ui.NewDiv(jaws.MakeHTMLGetter("content")))
rw.UI(ui.NewCheckbox(myBoolSetter), "disabled")
rw.UI(ui.NewRange(myFloatSetter))
```

## Building blocks

- `HTMLInner`
  - For tags like `<div>...</div>`, `<span>...</span>`, `<td>...</td>`.
- `Input`, `InputText`, `InputBool`, `InputFloat`, `InputDate`
  - For interactive inputs with typed parse/update behavior.
- `WrapContainer`
  - For widgets that render and maintain dynamic child lists.

## Adding a simple static widget

Use `HTMLInner`:

```go
type Article struct{ ui.HTMLInner }

func NewArticle(inner jaws.HTMLGetter) *Article {
  return &Article{HTMLInner: ui.HTMLInner{HTMLGetter: inner}}
}

func (w *Article) JawsRender(e *jaws.Element, wr io.Writer, params []any) error {
  return w.renderInner(e, wr, "article", "", params)
}
```

## Adding an interactive input widget

Use one of the typed input bases:

- `InputText` for string-based inputs
- `InputBool` for boolean inputs
- `InputFloat` for numeric inputs
- `InputDate` for `time.Time` inputs

Each base handles:

- tracking last rendered value
- receiving `what.Input`
- applying dirty tags on successful set
- update-driven `SetValue` pushes

## Adding a container widget

Use `WrapContainer`:

```go
type UList struct{ ui.WrapContainer }

func NewUList(c jaws.Container) *UList {
  return &UList{WrapContainer: ui.NewWrapContainer(c)}
}

func (w *UList) JawsRender(e *jaws.Element, wr io.Writer, params []any) error {
  return w.RenderContainer(e, wr, "ul", params)
}

func (w *UList) JawsUpdate(e *jaws.Element) {
  w.UpdateContainer(e)
}
```
