# Widgets and templates

[Documentation](../README.md) · [Bindings](../bindings.md) · [Dependency tags](../tags.md) · [Examples](../examples.md)

The [`ui` package](../../lib/ui) renders Go values as interactive HTML. Start with
the [application setup](../getting-started.md), then choose a [binding](../bindings.md)
for each widget's content or value.

Use ordinary HTML and template interpolation for content that stays fixed after
construction. Add JaWS widgets where values or browser events need live handling.

| What to render | Building blocks |
| --- | --- |
| Updating text or HTML | [`Span`, `Div`, `Button`, and the other `HTMLInner` widgets](../bindings.md#compute-content-and-attributes) |
| Editable values | [`Text`, `Password`, `Textarea`, `Checkbox`, `Radio`, `Date`, `Number`, `Range`](controls.md#input-controls) |
| Changing child lists | [`Container`, `Tbody`, `Select`, `MultiSelect`](controls.md#lists-and-selections) |
| Go templates | [`Handler`, `Template`, `RequestWriter`](#render-a-page-or-partial) |
| Initial attributes | [Constant template strings and member functions](#choose-initial-attributes) |
| A browser JavaScript variable | [`JsVarStore` and a per-request binding](jsvar.md#bind-javascript-variables) |

For extensions, see [custom widgets](custom.md#write-a-custom-widget).

## Render a page or partial

Use `ui.Handler(jw, "page", dot)` for a complete document. JaWS passes a `ui.With`
value to the template: `.Dot` holds your data, and `$.Span`, `$.Text`, and the
other helpers render widgets through its `RequestWriter`.

Pass your application object as `dot`. Templates can call its exported methods
directly to obtain bindings, content, attributes, and tags. Methods reading
mutable state use the application's lock.

```gotemplate
<!doctype html>
<html>
  <head>{{$.HeadHTML}}</head>
  <body>
    {{template "name-fields" .}}
    {{block "footer" .}}<footer>My application</footer>{{end}}
    {{$.TailHTML}}
  </body>
</html>
```

`Handler` creates a fresh Request for each page load and sets
`Cache-Control: no-store`. Its dot is reused, so shared data and callbacks must
support concurrent execution. The page dot can be arbitrary Go template data.
See [runtime](../runtime.md) for connection callbacks and custom page handlers.

Use standard Go template mechanisms for composition and render-time logic:
`define` and `template` for reusable fragments, `block` for a default fragment
that a template set can override, `if` and `with` for conditions, and `range`
for iteration whose child set, order, and identity stay fixed after construction.
Local variables keep a value available for reuse. For the page above, define
these fragments in the same template set:

```gotemplate
{{define "name-fields"}}
  {{with .Dot}}
    {{$name := .NameBinding}}
    {{$.Span $name}}
    {{if .ShowEditor}}{{$.Text $name}}{{end}}
    {{template "links" .Links}}
  {{end}}
{{end}}

{{define "links"}}
  <nav>{{range .}}<a href="{{.URL}}">{{.Title}}</a>{{end}}</nav>
{{end}}
```

At the page root, `{{template "name-fields" .}}` passes `ui.With`, including its
helpers. Pass `.Dot` or a field beneath it when a fragment expects only
application data. Inside each invoked template, `$` starts at the value passed
to that invocation; caller variables are not inherited. The `links` fragment
receives only the link slice, so it has no JaWS helpers. The example assumes this
link list stays fixed after construction. `if` preserves dot; `with` and `range`
change it within their bodies.

The [content and attributes example](../bindings.md#compute-content-and-attributes)
uses `with` to call `.NameUI` and member tag methods on the current user, keeping
widget choice, constant attributes, and dependency tags in the template.

Ordinary template actions run only when their containing template renders.
Changing Go state does not reevaluate an `if`, rerun a `range`, or update plain
`{{.Field}}` output in an existing page. Widgets created by those actions can
still update independently through their bindings.

Use `$.Template` when a region needs its own dependency tags and must rerun
its template actions after state changes:

```gotemplate
{{$.Template "section" "details" .Dot.Details}}
```

It renders a named partial inside an addressable wrapper. An empty wrapper
argument selects `div`; use `tr`, `td`, `li`, or another suitable tag when the
DOM context requires it. The partial receives a fresh `ui.With`: `.Dot` is the
supplied data and `$` exposes the JaWS helpers. The Template takes dependency tags
from the partial's dot and delegates input, click, and context-menu events to it.
The dot must be nil or comparable at runtime, equal to itself, and usable through
`tag.TagExpand`. A `JawsGetTag` method does
not make an otherwise non-comparable dot comparable.

Use a [Container](controls.md#lists-and-selections), `Tbody`, `Select`, or `MultiSelect`
when iterated children can be added, removed, reordered, or replaced after
construction. These reconcile child definitions and retain equal children's
Elements. Native `template` and `block` actions add no independently updateable
wrapper.

A JaWS Template owns the Elements created through its RequestWriter. A successful
update unregisters its previous Elements and sends new inner HTML. When that
HTML differs, the browser replaces the contents, including ordinary HTML controls,
losing their focus and unsent edits. To preserve a control, use a widget whose update changes that
Element's content or attributes while retaining its DOM node.
A failed update keeps the existing DOM and Elements and unregisters the failed
attempt's new Elements. Template execution can still produce queued messages or
application side effects before failing; it is not a transaction.

`NewTemplate` accepts trusted raw attribute strings for its wrapper. Render
parameters take precedence over constructor attributes, which take precedence
over attributes returned by the dot's `JawsInitialHTMLAttr` method. Initial
attributes run once per rendered wrapper. Updates replace only its inner HTML.
To update wrapper attributes, embed `ui.Template` in a custom widget and override
`JawsUpdate`: call the embedded updater for content and use `Element.SetAttr` or
`RemoveAttr` for attributes. See [custom widgets](custom.md#write-a-custom-widget) and
[HTML and attribute safety](../bindings.md#html-and-attribute-safety).

## Choose initial attributes

Ordinary template conditions handle initial attributes on template-authored HTML:

```gotemplate
<section{{if .Dot.Highlight}} class="highlight"{{end}}>Content</section>
```

**For widgets, pass static attributes as constant strings in the template.**
For state-dependent initial attributes, **pass a member function's
`template.HTMLAttr` result as another parameter**, just like tags:

```go
func (u *User) NameAttrs() template.HTMLAttr {
    u.mu.RLock()
    defer u.mu.RUnlock()
    if !u.canEditName {
        return "readonly"
    }
    return ""
}
```

```gotemplate
{{with .Dot}}
  {{$.Text .NameBinding `class="username"` .NameAttrs}}
{{end}}
```

`template.HTMLAttr` is trusted attribute syntax. Return trusted literals, as
above, or use [`htmlio.Attr`](../utilities.md#write-html) with a trusted name to
escape an untrusted value. Keep simple attribute conditions in templates or
these methods. `JawsInitialHTMLAttr` is an alternative when the source itself
should supply initial attributes.

These expressions run when the template renders. Changing their source state
does not update an existing attribute. For later changes, use
[a getter or an overridden `JawsUpdate`](custom.md#update-attributes-after-rendering),
or rerender a containing region that replaces the affected element.

## Widget lifetime and identity

Construct fresh widgets for each Request.
RequestWriter helpers do this for you; explicit construction uses
`rw.NewUI(ui.NewText(binding), params...)`.
Widgets may share synchronized application state, binders, handlers, and tags.
Every non-nil `jaws.UI` value must be comparable at runtime and equal to itself.
Keep slice-, map-, and function-bearing state behind stable pointers.

Render parameters supply trusted attributes (`string` or `template.HTMLAttr`),
input/click/context-menu handlers, and dependency tags. A handler can also
supply tags. See [actions and bindings](../bindings.md).

A widget normally backs one live Element. Input widgets retain per-control
state and need separate widget instances even when they share a binding:

```go
left := ui.NewText(nameBinding)
right := ui.NewText(nameBinding)
```

Within one Request, `HTMLInner` widgets, `Img`, and `Option` can back multiple
Elements when their sources support that use. `Template`, `Container`, `Tbody`, `Select`, and
`MultiSelect` can also do so under their provider and child contracts. Their
constructors return definition values: use those values directly, without
taking their addresses.

An Element belongs to its Request. Do not retain Elements or Requests in shared
application state or background work. A nil UI interface passed directly to
`Request.NewElement` renders and updates as a no-op; container child lists must
contain non-nil, comparable, reflexive UI values. A typed nil invokes its
concrete methods normally.
