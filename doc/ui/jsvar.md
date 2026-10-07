# Bind JavaScript variables

[Documentation](../README.md) · [Widgets and templates](README.md) · [Bindings](../bindings.md) · [Dependency tags](../tags.md)

`JsVarStore` keeps one authoritative Go value synchronized with a browser path
under `window`. Create the store once, configure it before use, and render a new
binding once per Request during its initial page render:

```go
store, err := ui.NewJsVarStore(jw, "client", &mu, &client)
if err != nil {
    return err
}
store.ClientCheck = func(source *jaws.Element, next *Client, path string) error {
    return validateClient(source, next, path)
}
```

Render the application object's `ClientStore`:

```gotemplate
{{$.NewUI (.Dot.ClientStore.Bind)}}
```

Keep this binding outside regions that may be replaced or removed. Render it on
the Jaws instance used to construct the store. Each browser path can be bound
only once per Request; paths such as `client` and `client.x` overlap and conflict.
For dotted names, parent objects must exist on `window` before jaws.js runs.
Top-level names need no existing property. Initial data and updates assign to
the live path, so existing JavaScript property setters may run.

Use `jawsVar("client.name", "Ada")` to propose a browser write. Accepted changes
update every binding; rejected proposals correct the originating binding. See the
[browser interface](../browser.md#read-and-write-javascript-values) for read,
write, and connection behavior.

`ClientCheck` is required for browser writes; nil denies them. It receives the
complete tentative Go value under the store's write lock. Validate authorization
through the originating Element and validate the full value, including fields
omitted from JSON and changes through parent or root paths. A `null` proposal for
a struct zeroes all its fields, including unexported and `json:"-"` fields. An
error or panic rolls the proposal back. The callback must only inspect tentative
data: do not mutate or retain it, reacquire the lock, or call a store setter.

Rejected proposals are logged and corrected without an automatic browser alert.
When handling browser input, `ClientCheck` can call
`source.Request.Alert("warning", "Choose a valid value.")` before returning an
error. A panic still produces a generic event-handler alert.

Every binding receives the same JSON value; use separate stores for data with
different visibility. Proposal outcomes can depend on Go fields omitted from
JSON, even when the JSON values are identical. Keep secrets outside the bound
value.

Browser JSON numbers are converted to the destination Go type before validation.
Conversion can truncate or wrap values: `2.9` becomes `2` for `int`, and `300`
becomes `44` for `uint8`. Use string fields for exact wide values or Number for
typed numeric input. The [JsVarBinding.JawsInput contract](../../lib/ui/jsvar.go)
details conversion limits.

Server writes use `SetPath`, `DeletePath`, or `WriteLocked`. Read with
`ReadLocked` or the supplied locker. `WriteLocked` groups edits under one lock;
successful edits remain applied if a later edit fails or the callback panics.
Callbacks must not retain borrowed data or re-enter lock-taking store methods.
Configure `ExtraTags` before use to dirty additional dependency tags after
changed writes.

Paths use dot-separated JSON names; an empty path replaces the root.
Keep values JSON encodable with unique object member names. Separately addressed
paths used for partial map patches must not share mutable aliases.
`JSONSizeCheck` bounds encoded bytes, not Go heap capacity; a size rejection
cancels the originating Request. See [JsVarStore's API](../../lib/ui/jsvar.go) for path,
patch, and callback contracts.
