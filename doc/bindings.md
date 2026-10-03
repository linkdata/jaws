# Bind values to widgets

[Documentation](README.md) · [Widgets and templates](ui/README.md) · [Dependency tags](tags.md)

A widget chooses the HTML element. Its source supplies the content or value.
The [`bind` package](../lib/bind) adapts application values to those sources.

Use your Go objects directly as sources. Bind their editable fields with
`bind.New`, and add content, event, or tag methods to the objects as needed.

| Source need | Use |
| --- | --- |
| Read and write a field protected by a lock | `bind.New(&mu, &value)`. The backing pointer is its dependency tag. |
| Compute read-only text | `bind.StringGetterFunc(fn, tags...)`. HTML widgets escape the returned text. |
| Compute trusted HTML | `bind.HTMLGetterFunc(fn, tags...)`. The returned HTML is emitted unchanged. |
| Add actions to HTML content | `ui.New(content).Clicked(...)` or `.ContextMenu(...)`. Pass the resulting Object to an HTML widget. |
| Supply initial attributes | Pass constant strings for static attributes; a member function returning `template.HTMLAttr` for [state-dependent attributes](ui/README.md#choose-initial-attributes). |
| Supply a custom source | Use `bind.HTMLGetter` for HTML content with event handlers, `bind.Getter[T]` for value-only reads, or `bind.Setter[T]` for inputs. |

Implement `JawsGetHTML` on a custom HTML source to preserve its click and
context-menu methods when used as the widget's first argument.
Adapting a plain `Getter[string]` escapes its text and preserves tags, but does not
forward its event handlers. Alternatively, pass the handler explicitly as a
separate widget parameter.

`ui.New(content)` retains the adapted getter's HTML and `JawsGetTag` result.
An existing HTML getter without `JawsGetTag` contributes no tag. Source events
and initial-attribute hooks are not inherited; add them with Object builders.

## Bind a field

Return a field binding from the application object:

```go
type User struct {
    mu   sync.RWMutex
    name string
    canEditName, highlighted bool
}

func (u *User) NameBinding() bind.Binder[string] {
    return bind.New(&u.mu, &u.name)
}
```

Pass the existing `user` directly to `ui.Handler(jw, "user", user)`. Its template
uses the method for display and editing:

```gotemplate
{{$.Span .Dot.NameBinding}}
{{$.Text .Dot.NameBinding}}
```

Each helper constructs its own widget. Both widgets depend on `&user.name`.
A changed or rejected browser edit dirties that tag. For a server-side change,
hold the same lock while changing the value, then call `jw.Dirty(&user.name)` after
unlocking. See [dependency tags](tags.md) for update scope.

The bound type must be strictly comparable: it cannot be an interface, contain
interface fields, or contain slices, maps, or functions. A `sync.RWMutex` enables
read locks; a locker with only `Lock` and `Unlock` uses exclusive locking for
reads as well. All application access to the value must use the same lock.

## Validate, format, and handle events

Binder builders return new chains. They share the original locker, pointer, and
dependency tag; adding a hook leaves earlier chains unchanged. Extend the same
binding method to validate edits:

```go
func (u *User) NameBinding() bind.Binder[string] {
    return bind.New(&u.mu, &u.name).
        SetLocked(func(prev bind.Binder[string], elem *jaws.Element, next string) error {
            if strings.TrimSpace(next) == "" {
                return errors.New("name is required")
            }
            return prev.JawsSetLocked(elem, strings.TrimSpace(next))
        })
}
```

`SetLocked`, `GetLocked`, and `InitialHTMLAttr` run under the binder's lock and
receive the previous binder in the chain. Delegate through its locked methods.
Do not reacquire the lock or call the public locking getter or setter there.
The default setter stores a changed value and returns `jaws.ErrValueUnchanged`
when the value is equal.

Returned errors use [browser alerts](browser.md#connection-loss).
Render a bound application error message for inline feedback.

`GetHTML` also runs under the lock, but receives the binder on which rendering
was invoked. Read it with `JawsGetLocked`. The newest `GetHTML` or `Format` hook
wins. Default rendering and `Format` escape their output; `GetHTML` returns
trusted HTML.

`Clicked` and `ContextMenu` run without the binder lock, newest first. Return an
error matching `jaws.ErrEventUnhandled` to continue to an older hook. They receive
the complete invoked chain. `Success` hooks run newest first after a successful
set and after unlocking; the first error stops the remaining success hooks.
See the [hook example](../lib/bind/example_test.go) and
[Binder API](../lib/bind/binder.go) for callback signatures.

Keep an action on the object whose state it changes:

```go
type Counter struct {
    mu    sync.Mutex
    count int
}

func (c *Counter) CountBinding() bind.Binder[int] {
    return bind.New(&c.mu, &c.count)
}

func (c *Counter) IncrementAction() ui.Object {
    return ui.New("Add one").Clicked(func(_ ui.Object, e *jaws.Element, _ jaws.Click) error {
        c.mu.Lock()
        c.count++
        c.mu.Unlock()
        e.Dirty(&c.count)
        return nil
    })
}
```

Pass `counter` directly to `ui.Handler(jw, "counter", counter)`:

```gotemplate
{{$.Button .Dot.IncrementAction}} {{$.Span .Dot.CountBinding}}
```

## Compute a value

A method can return a getter derived from the object's state:

```go
func (u *User) Greeting() bind.Getter[string] {
    return bind.StringGetterFunc(func(*jaws.Element) string {
        u.mu.RLock()
        defer u.mu.RUnlock()
        return "Hello, " + u.name
    }, &u.name)
}
```

Render it with `{{$.Span .Dot.Greeting}}`. Supply every dependency needed to
refresh it. The adapters copy the top-level tag slice, but
nested containers and referenced values must retain stable identities.

## Compute content and attributes

Keep layout, constant attributes, and dependency selection in the template:

```gotemplate
{{with .Dot}}
  {{with .CurrentUser}}
    {{$.Span .NameUI `class="username"` .NameTag .PermissionsTag .HighlightTag}}
  {{end}}
{{end}}
```

`with` changes dot to the application object and then its current user; `$`
retains the JaWS helpers. The member methods supply the getter and tags:

```go
func (u *User) NameTag() any        { return &u.name }
func (u *User) PermissionsTag() any { return &u.canEditName }
func (u *User) HighlightTag() any   { return &u.highlighted }

func (u *User) NameUI() bind.HTMLGetter {
    return bind.HTMLGetterFunc(func(e *jaws.Element) template.HTML {
        u.mu.RLock()
        name, canEdit, highlighted := u.name, u.canEditName, u.highlighted
        u.mu.RUnlock()

        if canEdit {
            e.SetAttr("title", "Editable name")
        } else {
            e.RemoveAttr("title")
        }
        if highlighted {
            e.SetClass("highlight")
        } else {
            e.RemoveClass("highlight")
        }
        return template.HTML(template.HTMLEscapeString(name))
    })
}
```

The getter runs during rendering and each update. Class operations preserve the
template's `username` class; both branches handle changes in either direction.
**The getter queues attribute and class commands during initial rendering too.**
`TailHTML` or the WebSocket applies them; they are absent from the initial HTML.
If initial state must appear in the HTML, pass [initial attributes](ui/README.md#choose-initial-attributes)
as well. Keep the getter for later changes; duplicate initial commands are
usually acceptable. A [custom update](ui/custom.md#update-attributes-after-rendering)
can avoid them when measured cost justifies it.
The escaped name is written as the Span's content.
After changing state under the same lock and unlocking, dirty its member tag, such as
`jw.Dirty(myUser.PermissionsTag())`. The template has already registered all
three dependencies; this getter needs no tags of its own.

## HTML and attribute safety

Constant strings are common in JaWS templates: `class="username"` above is a
normal string supplying trusted attribute syntax. Likewise,
`{{$.Span "Hello"}}` supplies trusted HTML content. Neither needs a
`template.HTML` or `template.HTMLAttr` conversion.

**Do not pass user-provided strings directly as JaWS HTML content or raw
attribute parameters.** Go template autoescaping does not protect values passed
into these helpers. Use an escaping binding or string getter for text, or escape
it before returning `template.HTML`, as `NameUI` does above. Use `htmlio.Attr`
for attribute syntax containing untrusted values.

HTML widgets call [`MakeHTMLGetter`](../lib/bind/makehtmlgetter.go), which uses
the first matching conversion:

| Source | HTML output |
| --- | --- |
| `bind.HTMLGetter` | Used unchanged. Its output is trusted HTML. |
| `template.HTML` | Used unchanged. |
| `bind.Binder[string]` without `JawsGetHTML`, or `bind.Getter[string]` | Escaped `JawsGet` output. |
| `fmt.Stringer` | Escaped `String` output. |
| Plain `string` | Used unchanged as trusted HTML. |
| Other values | Escaped `fmt.Sprint` output. |

A plain string passed to `ui.NewSpan` or `ui.New` is HTML, not escaped text.
Binders created by `bind.New` implement `HTMLGetter`, so their default, `Format`,
and `GetHTML` rules apply before the string-binder adapter.

A wrapper embedding `Binder[string]` without `JawsGetHTML` renders its own
`JawsGet` output; embedded HTML hooks do not run. Existing `HTMLGetter` values
and the string-binder adapter retain event and initial-attribute methods.
Other conversion adapters do not. Getter and Stringer adapters expose the
wrapped source as an implicit tag, so it must be a usable tag or implement
`JawsGetTag` to expose tags or return nil.

String and `template.HTMLAttr` render parameters are trusted raw attributes.
Build an attribute containing untrusted data with
[`htmlio.Attr`](utilities.md#write-html), using a trusted attribute name. For
`NewTemplate` constructor attributes, convert the result to `string`.
`Element.SetAttr` instead takes an unescaped logical value; it does not parse
HTML attribute syntax. Keep attribute names trusted.

## Custom input sources

A writable source must expose stable dependency tags for accepted, rejected, or
normalized browser edits. A binder supplies its backing pointer as its tag. A
custom setter can be pointer-valued or implement `JawsGetTag` with at least one
usable tag. `JawsGetTag` takes precedence over the setter's own identity.

Input handling dirties the source's tags. Tags passed as widget render parameters
register additional dependencies but are not dirtied by input handling. Editable
`Number` and `Range` reject a source without usable tags during rendering. Other
input bases cannot automatically reconcile rejected or normalized input without
them.

`MakeGetter[T]` accepts a getter or a static value. `MakeSetter[T]` also accepts
a getter or static value, but those adapters return `bind.ErrValueNotSettable`
when written. They still implement `Setter`, so numeric widgets treat them as
editable sources. Pass a getter directly, or use `MakeGetter`, for a read-only
numeric control. Adapter constructors panic on unsupported dynamic types.

`MakeSetter` returns an existing Setter unchanged. When it adapts a Getter, it
preserves its tags but not event or initial-attribute methods. Pass handlers and
initial attributes as render parameters, or implement a Setter with those methods.

Continue with [input widgets](ui/controls.md#input-controls) or
[dependency tags](tags.md).
