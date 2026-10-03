# Input controls

[Documentation](../README.md) · [Widgets and templates](README.md) · [Bindings](../bindings.md) · [Dependency tags](../tags.md)

Use a [binder](../bindings.md#bind-a-field) or custom setter that exposes stable
dependency tags for editable controls. The standard input bases dirty those tags
after a setter result other than `jaws.ErrValueUnchanged`, so changed, rejected,
or normalized values can be read back from Go state. Tags passed as render
parameters register additional dependencies but are not dirtied by input handling.

`Text`, `Password`, and `Textarea` bind strings; `Checkbox` and `Radio` bind
booleans; `Date` binds `time.Time`. A date control shows the value's calendar date
in its own location, but browser edits store midnight UTC. An empty edit stores
the zero time, which renders as `0001-01-01`. Use dates in years 1 through 9999
and keep the bound time and location irrelevant or normalized to UTC.

`Number` and `Range` accept predeclared or named integer and floating-point
types. They preserve the source type for parsing, formatting, and setter calls:

```go
type Percent uint8

percent := Percent(50)
percentBinding := bind.New(&mu, &percent)
```

Pass `percentBinding` as the dot to `ui.Handler(jw, "percent", percentBinding)`:

```gotemplate
{{$.Number .Dot `step="1"`}}
{{$.Range .Dot `min="0"` `max="100"` `step="1"`}}
```

A numeric source is editable when it implements `bind.Setter[T]`; an editable
source without a usable dependency tag fails rendering. Getter-only Number
controls are readonly;
getter-only Range controls are disabled and omitted from native form submission.
Static values passed to the template helpers use these getter-only forms.

Number sends edits on `change`. Range sends live `input` events. Invalid or
unrepresentable numeric text does not reach the setter; JaWS restores canonical
text on the originating control. Accepted text is also normalized when the
setter reports an unchanged value. A non-finite bound float cancels the Request.
Range supplies no `min`, `max`, or `step`; provide attributes when browser
defaults do not fit the source domain.

See [connection readiness](../browser.md#events-and-connection-readiness) for
gating controls until the WebSocket opens; earlier interactions are not replayed.

Native form reset changes browser state without updating Go bindings. Implement
reset as a JaWS-handled `type="button"` action that changes authoritative Go
values and dirties their tags; `Button` supplies `type="button"` automatically.
Independently bound Radio widgets do not form a
server-side group merely by sharing an HTML name.

## Lists and selections

Implement `jaws.Container.JawsContains` to return child UI definitions, then
render them with `Container` or `Tbody`. The provider must be comparable and
equal to itself. This provider keeps its mutable list behind a locked pointer;
each item's name remains immutable after construction:

```go
type Item struct{ Name string }

type ItemList struct {
    mu     sync.RWMutex
    values []*Item
}

func (items *ItemList) JawsContains(*jaws.Element) []jaws.UI {
    items.mu.RLock()
    defer items.mu.RUnlock()
    children := make([]jaws.UI, len(items.values))
    for i, item := range items.values {
        children[i] = ui.NewTemplate("li", "item", item)
    }
    return children
}
```

Pass `items := &ItemList{}` directly to `ui.Handler`:

```gotemplate
{{$.Container "ul" .Dot}}
{{define "item"}}{{.Dot.Name}}{{end}}
```

After adding, removing, or reordering items, unlock and dirty the list tag
(`items` here):

```go
items.mu.Lock()
items.values = append(items.values, &Item{Name: "Ada"})
items.mu.Unlock()
jw.Dirty(items)
```

The provider pointer is the container's dependency tag. Its returned child slice
becomes read-only after return. Rebuilding a Template with the same wrapper tag,
template name, attributes, and item pointer produces an equal definition.
Reconciliation retains the original Element and UI for equal children; newly
allocated `NewSpan` pointers would be unequal on every call.

Each child must render one direct DOM node with its Element's JaWS ID;
`NewTemplate` supplies that wrapper for partial templates.
Retained children are not updated just because their parent reconciles. A child
with changing content needs its own dependency tag and dirty call; the example's
Templates use each item pointer as that tag. Moving a child between parents
creates a different Element. A nested container whose contents change needs its
own dirty/update pass.

For editable rows, use stable child Template definitions and bind their inputs
to fields. Updating a row Template recreates its nested Elements and can discard
focus or unsent edits; dirty the field's tag when only its value changed.

Equal containers may back several Elements only when the provider and any
reused child widgets support that use. A repeated input widget pointer does not;
construct separate inputs instead. Container, Template, and selection widgets
each claim one Element state slot, so two state-owning renderers cannot share
one Element.

Use [`named.BoolArray`](../utilities.md#named-choices) for standard selection data:

- `named.NewBoolArray(false)` or its zero value supports `Select` and
  `RequestWriter.RadioGroup` with one selection.
- `named.NewBoolArray(true)` supports `MultiSelect` with several selections.
- Keep option names non-empty and distinct. Do not add the HTML `multiple`
  attribute to `Select`; use `MultiSelect`.

```gotemplate
{{$.Select .Dot.Choice}}
{{$.MultiSelect .Dot.Choices}}
{{range $.RadioGroup .Dot.Choice}}
  {{.Radio}} {{.Label}}
{{end}}
```

Call `RadioGroup` from the template that renders its results. Render each
`Radio` and `Label` at most once, and render a label only with its radio.
The group supplies a request-specific HTML name and keeps Go selection state
synchronized. This `range` example assumes the option list stays fixed after
construction.

MultiSelect replaces the complete selection with a JSON string array. An empty
server-side selection clears every option. Malformed browser input never
reaches its handler; rejected, unchanged, and malformed input all reconcile the
originating control. Options are rendered or reconciled before the selected
values are applied.

Initial child-render errors return to the caller. Append errors during an
update go through `MustLog`, which may panic without a logger. Failed new
children are unregistered and omitted from the browser; other completed update
steps remain applied.

## Browser message size

Each browser-to-server WebSocket message must fit the 32 KiB inbound limit,
including protocol and JSON overhead; widgets do not chunk payloads, and an
oversized message closes the connection. See
[transport limits](../transport.md#limits-and-connection-loops) and
[deployment](../deployment.md) for handling larger data.
