# Write a custom widget

[Documentation](../README.md) · [Widgets and templates](README.md) · [Bindings](../bindings.md) · [Dependency tags](../tags.md)

## Update attributes after rendering

A [getter](../bindings.md#compute-content-and-attributes) can update attributes
as well as content. It also runs during initial rendering and queues those
attribute commands. If initial attributes must appear in markup, pass them
[as parameters](README.md#choose-initial-attributes) and keep the getter for
later changes. A JaWS Template wrapper needs a `JawsUpdate` override to change
its attributes because its update replaces only inner HTML. If measured cost
justifies avoiding the getter's initial commands, put initial attributes in
markup and handle later changes in `JawsUpdate` instead of a getter. Call the
embedded updater if its content or value still needs updating. Handle attribute
changes in both directions with `Element.SetAttr` and `RemoveAttr`.

## Render custom markup

First compose standard widgets with [application methods](../bindings.md#compute-content-and-attributes).
For custom element markup, embed `HTMLInner` and provide its render method:

```go
type Article struct{ ui.HTMLInner }

func NewArticle(inner any) *Article {
    return &Article{HTMLInner: ui.HTMLInner{HTMLGetter: bind.MakeHTMLGetter(inner)}}
}

func (u *Article) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
    e.ApplyGetter(u.HTMLGetter)
    getterAttrs := e.ApplyInitialHTMLAttr(u.HTMLGetter)
    attrs := append(e.ApplyParams(params), getterAttrs...)
    return htmlio.WriteHTMLInner(w, e.Jid(), "article", "", u.HTMLGetter.JawsGetHTML(e), attrs...)
}
```

`ApplyGetter` registers source tags and event handlers. Initial attributes need
the separate `ApplyInitialHTMLAttr` call; do not hold a lock that its callback
may acquire. The embedded `JawsUpdate` reads the getter and sends the inner HTML
on each update. Getters should read domain state without changing it.

For a custom input type, embed `InputText`, `InputBool`, or `InputDate` and call
its `RenderInput` from `JawsRender`. The base supplies input and update methods:

```go
type Email struct{ ui.InputText }

func (u *Email) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
    return u.RenderInput(e, w, "email", params...)
}
```

Construct it with
`&Email{InputText: ui.InputText{Setter: bind.New(&mu, &value)}}`.
`RenderInput` emits an `<input>`; Textarea has its own renderer.

Use Element DOM helpers such as `SetAttr`, `SetInner`, `SetValue`, `Append`, and
`Remove` during render/update processing. After an event changes application
state, dirty its dependency tags or pass an exact Element to `Dirty` to schedule
that processing.

## Register template-authored HTML

`RequestWriter.Register` connects a render-independent `jaws.Updater` to static
HTML written by the surrounding template:

```gotemplate
<section id="{{$.Register .Dot.Panel}}" class="panel">
  template-authored content
</section>
```

Always emit the returned Jid as the element's `id`. Register tags the Element,
attaches handlers, and calls `JawsUpdate` once; it never calls `JawsRender`.
Attribute parameters are ignored, so write attributes in the template.
DOM changes from that initial update are queued. `TailHTML` can apply queued
attribute and class changes before connection; queued content waits for the
WebSocket. Use a standard HTML widget for complete initial content. Its
initial and later attributes can use the [patterns above](#update-attributes-after-rendering).

The updater must be comparable, equal to itself, usable as a tag, and work
without render-time initialization. Register creates a fresh widget adapter;
the updater itself may be shared across Requests when it keeps no per-Element
state and supports concurrent calls. Registered HTML should contain no JaWS
widgets; render them through normal helpers.
