# Browser client and resources

[Documentation](README.md) · [UI](ui/README.md) · [Bindings](bindings.md) · [Transport](transport.md)

JaWS sends rendered HTML to the browser, then uses a WebSocket to receive events
and update the page. Application state lives on the server. The embedded
[browser client](../lib/assets/jaws.js) manages elements with canonical `Jid.*`
IDs; application JavaScript can run alongside it.

## Load page resources

Call `jw.GenerateHeadHTML` with any extra resource URLs, check its error, and
render `HeadHTML` in the page's `<head>`. The generated head includes the JaWS
script, stylesheet, and request key. See [UI](ui/README.md) for a complete page and
[deployment](deployment.md) for serving and security headers.

```go
if err := jw.GenerateHeadHTML("/static/app.js", "/static/app.css"); err != nil {
    return err
}
```

Resource URLs are trusted application configuration. The resource helper emits:

| Resource classification | Generated markup |
| --- | --- |
| Matched `.js` | Deferred classic script |
| Matched `.css` | Stylesheet |
| Image | Image preload |
| Font | Font preload with anonymous CORS |
| Image whose base name starts with `favicon`, case-insensitively | Icon; the last qualifying URL wins |

Module scripts, MIME-only scripts or stylesheets, generic fetch resources, and
unrecognized URLs are omitted from automatic markup. Load these explicitly in
the template when needed, and configure the appropriate security policy.
`GenerateHeadHTML` also builds the content security policy from the parsed URLs.
It logs omitted resources when a logger is configured. Invalid URLs are omitted
and reported in its returned error; the remaining head and policy are installed.

The immutable embedded strings are exposed as `assets.JavascriptText` and
`assets.JawsCSS`. The [resource helper](../lib/assets/js.go) documents detailed
classification, including version suffixes.

## Events and connection readiness

The client opens `/jaws/<request-key>` after the document is parsed, using `wss`
on an HTTPS page. Browser events are sent only while the WebSocket is open.
Events that occur while connecting or disconnected are neither queued nor
replayed. Native controls may still change their local values.

For controls that require a connection, render them disabled or inert and clear
that state from the server after connection. See [request lifecycle](runtime.md)
for connection handlers.

Managed inputs, selects, and textareas send `Input` events. `Number` uses
`change`; `Range` and other managed controls use `input`. Checkboxes and radio
buttons send their checked state. A multiple-select control sends all selected
values as a JSON string array; a single-select control sends one string.

Managed non-input elements forward clicks and context-menu events through their
managed ancestor route. Events originating inside an input, select, textarea,
or option are left to native input handling. A forwarded context-menu event
suppresses the native menu. See [event handlers](bindings.md#validate-format-and-handle-events).

A handler attached to one control already knows its action. When dispatching
among several controls by `jaws.Click.Name`, give them explicit `name` attributes.
Otherwise the name comes from an ancestor's name, button text, or the target ID.
Treat it as browser input.

Native form reset does not produce the per-control events JaWS transports.
Implement a reset action by updating server values and dirtying their
[dependency tags](tags.md).

## DOM updates

Server HTML is inserted as trusted HTML. Escape untrusted text before turning it
into `template.HTML` or using a raw-string HTML binding. See
[bindings](bindings.md) and [HTML helpers](utilities.md#write-html).

The client attaches handlers to inserted managed nodes and reports removed
managed descendants to the server. Managed IDs cannot be changed through
attribute commands. Child insertions and removals target direct children only.
An insertion position can be a child Jid or a zero-based index of an existing
child; appending has its own command.

Value updates change live control properties. Unchanged values are left alone.
Text selection preservation is best effort: it handles changes where one whole
value is a contiguous substring of the other. Multiple-select updates reconcile
every option's selected state, including clearing the selection.

Ordinary command failures are logged and other commands in the same WebSocket
message continue. A failed JavaScript store patch closes the connection and
reloads the page.

## Read and write JavaScript values

`jawsVar("app.state")` reads the current value from `window`. If a store binding
covers that path and the connection is open, the read also attempts to submit
the value to the server. Serialization or send failures do not prevent the
local read.

```javascript
const state = jawsVar("app.state");
const sent = jawsVar("app.state", { expanded: true });
```

A two-argument call to a bound path sends one proposal and then assigns its JSON
value locally. It returns `false` without changing the local value if the socket
is closed or serialization/send fails. Unbound paths work locally. The parent
path must already exist, and invalid or reserved paths throw. Bound array writes
must target an existing canonical index. The server accepts or reconciles
proposals through the [JavaScript binding API](ui/jsvar.md).

Both reads and writes that submit bound values consume the Request's
[`MaxEventRate` budget](deployment.md#capacity).

Binding nodes provide the store name and initial JSON during the initial page
render. The live values then reside on `window`. Server patches can replace the
root, replace a subtree, or delete an object property.

## Connection loss

A server-requested reload stops connection recovery before reloading the page.
It does not show the connection-loss indicator.

After a WebSocket failure, the client waits five seconds before probing
`/jaws/.ping`. A successful probe reloads a page whose navigation age is at least
60 seconds. Otherwise, the client shows a connection-loss indicator and retries
with a delay based on elapsed time, capped at 60 seconds. Each probe has a
ten-second timeout, so a stalled first probe can defer the indicator until
roughly 15 seconds after failure.

The indicator uses `[data-jaws-lost]` if present, or creates one at the start of
the body. A `pagehide` event stops recovery for that document. A page restored
from the browser's back-forward cache reloads. Navigation that leaves the
document active can still show the indicator or trigger recovery before
`pagehide` occurs.

Alerts appear in an existing `[data-jaws-alerts]` container when Bootstrap is
present; if either is missing, the client writes them to the console. See the
[Bootstrap integration](bootstrap.md) for the supplied setup.
