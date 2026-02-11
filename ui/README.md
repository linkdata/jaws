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

### Regex find/replace

Use these for bulk migration in editors with regex capture-group support:

1. `jaws.UiX` -> `ui.X`

   Find:

   ```regex
   \bjaws\.Ui([A-Z][A-Za-z0-9_]*)\b
   ```

   Replace:

   ```text
   ui.$1
   ```

2. `jaws.NewUiX(...)` -> `ui.NewX(...)`

   Find:

   ```regex
   \bjaws\.NewUi([A-Z][A-Za-z0-9_]*)\(
   ```

   Replace:

   ```text
   ui.New$1(
   ```

3. Internal core package import path (`jaws/jaws` -> `jaws/core`)

   Find:

   ```regex
   "github\.com/linkdata/jaws/jaws"
   ```

   Replace:

   ```text
   "github.com/linkdata/jaws/core"
   ```

4. Handler helper move (`jw.Handler(name, dot)` -> `ui.NewHandler(jw, name, dot)`)

   Find:

   ```regex
   \b([A-Za-z_][A-Za-z0-9_]*)\.Handler\(
   ```

   Replace:

   ```text
   ui.NewHandler($1, 
   ```

5. Optional alias cleanup for core imports

   Find:

   ```regex
   ^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s+"github\.com/linkdata/jaws/core"\s*$
   ```

   Replace:

   ```text
   "github.com/linkdata/jaws/core"
   ```

### Command sequence

```bash
find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -q '"github.com/linkdata/jaws/jaws"' "$f" || continue
  sed -i 's#"github.com/linkdata/jaws/jaws"#"github.com/linkdata/jaws/core"#g' "$f"
done

find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -Eq '\bjaws\.NewUi[A-Z]|\bjaws\.Ui[A-Z]' "$f" || continue
  perl -i -pe 's/\bjaws\.NewUi([A-Z][A-Za-z0-9_]*)\(/ui.New$1(/g; s/\bjaws\.Ui([A-Z][A-Za-z0-9_]*)\b/ui.$1/g' "$f"
done

find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -Eq '\b[A-Za-z_][A-Za-z0-9_]*\.Handler\(' "$f" || continue
  perl -i -pe 's/\b([A-Za-z_][A-Za-z0-9_]*)\.Handler\(/ui.NewHandler($1, /g' "$f"
done

find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -q 'ui.NewHandler(' "$f" || continue
  grep -q '"github.com/linkdata/jaws/ui"' "$f" || \
    perl -0777 -i -pe 's@("github.com/linkdata/jaws"\n)@$1\t"github.com/linkdata/jaws/ui"\n@' "$f"
done

gofmt -w $(find . -name '*.go' -type f)
go test ./...
```

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
