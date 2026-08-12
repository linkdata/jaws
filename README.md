[![build](https://github.com/linkdata/jaws/actions/workflows/build.yml/badge.svg)](https://github.com/linkdata/jaws/actions/workflows/build.yml)
[![coverage](https://github.com/linkdata/jaws/blob/gitcoverage/main/badge.svg)](https://html-preview.github.io/?url=https://github.com/linkdata/jaws/blob/gitcoverage/main/report.html)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/linkdata/jaws/badge)](https://scorecard.dev/viewer/?uri=github.com/linkdata/jaws)
[![Docs](https://godoc.org/github.com/linkdata/jaws?status.svg)](https://godoc.org/github.com/linkdata/jaws)

# JaWS

JavaScript and WebSockets for creating responsive webpages.

JaWS embraces a "server holds the truth" philosophy and keeps the
complexity of modern browser applications on the backend. The
client-side script becomes a thin transport layer that faithfully
relays events and DOM updates.

## Features

* Moves web application state fully to the server.
* Keeps the browser intentionally dumb – no implicit trust in
  JavaScript logic running on the client.
* Binds application data to UI elements using user-defined tags and
  type-aware binders.
* Integrates with the standard library as well as third-party routers
  such as Echo.
* Ships with a small standard library of UI widgets and helper types
  that can be extended through interfaces.

There is a [demo application](https://github.com/linkdata/jawsdemo)
with plenty of comments to use as a tutorial.

## Installation

JaWS is distributed as a standard Go module. To add it to an existing
project use the `go get` command:

```bash
go get github.com/linkdata/jaws
```

After the dependency is added, your Go module will be able to import
and use JaWS as demonstrated below.

For widget authoring guidance see `lib/ui/README.md`.

### AI skill

This repository includes an AI skill under `.agents/skills/jaws/`.
To install it in your local AI skills tree, copy both `SKILL.md` and
`agents/openai.yaml` into `~/.agents/skills/jaws/`.

Using `curl`:

```bash
mkdir -p "$HOME/.agents/skills/jaws/agents"
curl -fsSL https://raw.githubusercontent.com/linkdata/jaws/main/.agents/skills/jaws/SKILL.md \
	-o "$HOME/.agents/skills/jaws/SKILL.md"
curl -fsSL https://raw.githubusercontent.com/linkdata/jaws/main/.agents/skills/jaws/agents/openai.yaml \
	-o "$HOME/.agents/skills/jaws/agents/openai.yaml"
```

## Quick start

The following minimal program renders a single range input whose value
is kept on the server. Copy the snippet into a new module, run `go
mod tidy`, and start it with `go run .`. Visiting
http://localhost:8080/ demonstrates the full request lifecycle.

```go
package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/ui"
)

const indexhtml = `
<html>
  <head>{{$.HeadHTML}}</head>
  <body>{{with .Dot}}
    {{$.Range .}}
  {{end}}{{$.TailHTML}}</body>
</html>
`

type Percent uint8

func main() {
	jw, err := jaws.New() // create a default JaWS instance
	if err != nil {
		panic(err)
	}
	defer jw.Close()           // ensure we clean up
	jw.Logger = slog.Default() // optionally set the logger to use

	// parse our template and inform JaWS about it
	templates := template.Must(template.New("index").Parse(indexhtml))
	_ = jw.AddTemplateLookuper(templates)

	go jw.Serve()                                 // start the JaWS processing loop
	http.DefaultServeMux.Handle("GET /jaws/", jw) // ensure the JaWS routes are handled

	var mu sync.Mutex
	percent := Percent(50)

	http.DefaultServeMux.Handle("GET /", ui.Handler(jw, "index", bind.New(&mu, &percent)))
	slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
}
```

Next steps when building a real application typically include:

1. Adding more templates and wiring them with `AddTemplateLookuper`.
2. Creating types that implement `JawsRender` and `JawsUpdate` so they
   can be used as widgets.
3. Introducing sessions (see below) to keep track of user state.

### Creating HTML entities

When JawsRender() is called for a UI object, it can call
NewElement() to create new Elements while writing their initial
HTML code to the web page. Each Element is a unique instance
of a UI object bound to a specific Request, and will have a
unique Jid-based HTML id such as `Jid.7`.

UI objects are request-scoped: construct fresh UI values for each Request and
never reuse one UI value across Requests. The application state, binders,
handlers and tags referenced by UI values may be shared when synchronized as
required. The `ui.RequestWriter` helpers construct fresh widgets while rendering.

Within a Request, a UI value normally backs one live Element. It may back
multiple live Elements only when its concrete type documents that support. To
show the same application state in several places, construct distinct widgets
that share the synchronized binder, getter, handler or tag.

If an HTML entity is not registered in a Request, JaWS will not
forward events from it, nor perform DOM manipulations for it.

Dynamic updates of HTML entities are queued with the methods on `Element` when
`JawsUpdate` is called. To reconcile browser-local state after an event, pass the
Element itself to `Dirty`; this schedules only that Element on its owning Request.
Dirty a shared application tag instead when every dependent Element must update.

### JavaScript events

Supported JavaScript events are sent to the server and
are first offered to any extra objects added to the Element, in
reverse registration order (last added first). If none handle the event, the Element's UI type is
invoked. If none handle the event, it is ignored.

The bundled client sends input, set, click, and context-menu messages only while
its WebSocket is open and does not queue them for later delivery.

Event handlers should return `ErrEventUnhandled` if they didn't
handle the event or want to pass it to the next handler.

* `onclick` invokes `JawsClick` for non-input-origin events
  (`val` as `x<SP>y<SP>keystate<SP>name`)
* `oncontextmenu` invokes `JawsContextMenu` for non-input-origin events
  (`val` as `x<SP>y<SP>keystate<SP>name`)
* `oninput` invokes `JawsInput`; editable `ui.Number` listens for `change`
  instead and sends the same Input event
* `what.Set` events invoke `JawsInput` (`val` as `path=json`)

Click and context-menu events whose target is an `input`, `select`,
`textarea` or `option` element, or inside one, are left to native input
handling and do not invoke ancestor click/context handlers.

### JavaScript variables

`ui.JsVar` binds a JSON-marshalable Go value to an application-owned variable
on the browser's `window`. It is intended for state that application JavaScript
must read or change and exchange with Go. Unlike most JaWS UI values, a `JsVar`
is a bidirectional channel: the binding does not by itself make either the Go
value or the browser value authoritative.

Browser JSON numbers use JavaScript `Number` values. Signed and unsigned
integers outside `-9007199254740991` through `9007199254740991` may be rounded,
and a later browser write may commit the rounded value to Go. Represent exact
wide integers as decimal fields of Go's built-in `string` type and convert them
explicitly, for example with `strconv.FormatInt` or `strconv.FormatUint`:

```go
type Client struct {
	Counter string `json:"counter"`
}
```

Convert JavaScript `BigInt` values back to strings before calling `jawsVar`:

```js
const next = BigInt(client.counter) + 1n;
client.counter = next.toString();
jawsVar("client.counter", client.counter);
```

A Go `json:",string"` tag is not generic round-trip support: browser input is an
untyped string that cannot be assigned to an integer field. Use a built-in
`string` field or implement `ui.PathSetter` to parse and validate it.

JSON-marshalable describes values JaWS can send to the browser. Generic browser
writes decode into an untyped Go value and do not invoke destination custom
unmarshaling. Consequently, `time.Time`, base64-encoded `[]byte`, and maps with
non-string Go keys are not round-trip writable by the generic setter. Use a
browser-facing DTO compatible with the generic setter, or implement
`ui.PathSetter` to parse and validate the decoded value.

Create each binding for the Request that renders it. A `JsVarMaker` can be kept
in shared handler data because each call returns a fresh `JsVar` over the
possibly shared backing state:

```go
type application struct {
	clientMu sync.Mutex
	client   Client
}

// JawsMakeJsVar creates the binding for one request.
func (app *application) JawsMakeJsVar(*jaws.Request) (ui.IsJsVar, error) {
	jsv := ui.NewJsVar(&app.clientMu, &app.client)
	jsv.ClientCheck = ui.JSONSizeCheck[Client](1 << 20) // 1 MiB
	return jsv, nil
}

app := new(application)
handler := ui.Handler(jw, "index", app)
```

```gotemplate
{{$.JsVar "client" .Dot}}
```

Several `JsVar` bindings may share a name. The name is a single `window`
property, and a browser-initiated write to it is delivered to every live binding
of that name; a removed binding simply stops receiving writes. This makes
re-rendering a subtree that contains a nested `JsVar` work, lets multiple requests
expose the same application-owned global, and lets one browser value fan out to
several independent Go bindings. When several bindings share the same backing
value, a browser write applies to it once per binding, so avoid exposing one
non-idempotent value through multiple simultaneously rendered bindings.

For a value that does not implement `ui.PathSetter`, an optional
`JsVar.ClientCheck` validates each actual browser-initiated change before it
commits. The check receives the complete tentative value and the
browser-supplied jq path while the application locker is held. The path is an
inspection hint, not an authorization key: it is passed through unchanged, jq
accepts equivalent noncanonical spellings with empty components, and both `""`
and `"."` address the root. Use `ui.PathSetter` when paths must be allow-listed.
Returning nil commits the change; returning an error rolls it back without
broadcasting it. If the error matches `ui.ErrJsVarTooLarge`,
`JawsInput` returns that sentinel and, during normal framework dispatch,
cancels the associated request after releasing the application locker; any
other error is returned without cancellation. A check must only inspect the
value: it must not mutate it, re-enter the `JsVar`, call a JSON path setter on
it, acquire the same locker, or retain references into a rejected tentative
value. It must not return or wrap `jaws.ErrEventUnhandled`, which would request
handler fallthrough rather than report a rejection.

The check validates tentative Go state, not the decoded browser value used in
the accepted peer broadcast. jq conversions and ignored map-to-struct entries
can make those values differ. Use `ui.PathSetter` when peer-visible input also
needs validation.

`ui.JSONSizeCheck[T](maxBytes)` provides an exact serialized-size policy. It
marshals the complete tentative value. A value exceeding `maxBytes`, or one
that cannot be marshaled, makes the check match `ui.ErrJsVarTooLarge`. A
non-positive limit disables the check. Time and allocation cost depend on the
whole value and its marshaling behavior; map-key sorting and custom marshalers
can add further cost. Use it when that cost is appropriate for the value and
threat model. It bounds `encoding/json` output, not Go heap use or backing-memory
size. Use it only when JSON faithfully represents all client-growable state;
custom `MarshalJSON` or `MarshalText` methods, omitted fields, aliases, or
collection capacity require a domain-specific `ClientCheck`. A nil
`ClientCheck` accepts every type-correct generic write and imposes no
accumulated-state size limit.

`ClientCheck` applies only to browser writes handled through the generic JSON
path setter. Server-initiated `JsVar.JawsSet` and `JsVar.JawsSetPath` calls
bypass it. A value implementing `ui.PathSetter` also bypasses it and must
allow-list paths and enforce collection or size limits in its own
`JawsSetPath`. Because the check belongs to a request-scoped binding, configure
an equivalent policy on every binding that exposes the same `Ptr` or reachable
mutable backing state to browser writes, and protect those bindings with the
same locker. One unchecked binding can otherwise modify the shared state
without validation.

`ClientCheck` is an acceptance gate, not a state monitor. It does not inspect
the initial render, server writes, invalid or unchanged generic writes, or
`PathSetter` writes. An ordinary rejection restores Go state and sends no
broadcast, but the originating browser has already changed its local value and
can remain divergent until the application resynchronizes it. A rejection
matching `ui.ErrJsVarTooLarge` instead cancels the associated request, when
present, and is terminal for that connection.

`jawsVar` and `JsCall` paths are application-controlled. The browser rejects
exact `__proto__` components; put user data in JSON values, not paths.

The name may refer to an existing application global. For example, browser
code can update that object and send either the complete value or one path:

```js
var client = {X: 0, Y: 0};

onmousemove = function (event) {
    client.X = event.clientX;
    client.Y = event.clientY;
    jawsVar("client"); // send the current complete value

    // Equivalently, set and send one path:
    // jawsVar("client.X", event.clientX);
};
```

When the `JsVar`'s `Ptr` is non-nil, rendering serializes its current Go value
into the binding element. When the JaWS script attaches that element, the
snapshot initializes the named browser variable. A browser call to `jawsVar`
sends only while the WebSocket is open; calls made earlier are not queued for
later transmission. On the Go side, successful `JawsSet` and `JawsSetPath`
calls change the bound value. Any broadcast they produce targets matching
active requests and is not replayed to a page that has rendered but has not yet
subscribed to broadcasts.

If either side can change the value between rendering and WebSocket setup, the
application must choose and implement a policy if it requires the two sides to
converge. Depending on the value, it may send the current browser state once
the connection is ready, resend the current Go state as part of an
application-level handshake, or merge selected paths according to application
rules. `JsVar` deliberately does not choose one of those policies.

## Technical notes

### WebSocket wire format notes

JaWS WebSocket protocol records are line-based and field-delimited:
`What<TAB>Jid<TAB>Data<LF>`. A text message may carry multiple records. Keep
these invariants in mind when changing client/server protocol code:

* The browser is not trusted. Incoming records are validated (`What`, `Jid`,
  delimiters, quoting), and malformed records are skipped independently.
* Each inbound WebSocket message is limited to 32 KiB. The bundled client does
  not chunk `Input`, `Set`, `Click`, `ContextMenu`, or `Remove`; an oversized
  message closes the connection. The limit covers the complete protocol payload
  after UTF-8 encoding, so there is no fixed maximum application-value length.
  Keep text values, click/context-menu names, and `jawsVar` writes conservatively
  sized; use HTTP uploads for large values and independently updated wrappers
  for large dynamic trees. `JsVar.ClientCheck` and `JSONSizeCheck` run after
  receipt and cannot enforce this transport limit.
* `what.Remove` means remove child element(s). For browser-originated `Remove`
  messages, the WebSocket `Jid` identifies the parent/container in the DOM and
  `Data` carries removed managed child IDs. The server only removes child IDs
  that are known in the current request.
* `what.Replace` replaces the target element HTML and carries plain HTML in `Data`.
* `what.Call`/`what.Set` use `path + "=" + json` inside `Data`. Paths may not
  contain tabs, newlines, carriage returns, or `=`. Embedded tabs or newlines in
  JSON break message framing; `Jaws.JsCall` compacts valid JSON before sending.
  A `Call` selected by a nil destination or nonzero request key uses a zero Jid
  and does not require a DOM element; a zero request key is dropped. Tag
  destinations remain element-scoped. Built-in strings and bare `Jid` values are
  not destinations: use `tag.Tag` or a domain tag for broadcasts, and `Element`
  methods for request-local operations.
* `jawsVar(name, ...)` resolves properties from `window`, so `JsVar` names share
  the page's global namespace. Use an application-owned name, including an
  existing global that browser code reads or changes. Do not bind a
  browser-owned property such as `window.name`, or a global owned by unrelated
  code: `JsVar` initialization and updates write that property. WebSocket
  routing uses the top-level symbol name only. Register names as top-level
  identifiers (for example, `app`), and use dotted suffixes as the JSON path
  (for example, `jawsVar("app.state", value)` sends path `state`). The exact
  top-level name `__proto__` is reserved; rendering it as a `JsVar` returns
  `ui.ErrIllegalJsVarName`. A name may be shared by several live bindings; a
  browser write is delivered to every live binding of the name. See
  [JavaScript variables](#javascript-variables) for the binding and
  synchronization model.

### HTTP request flow and associating the WebSocket

When a new HTTP request is received, create a JaWS Request using the JaWS
object's `NewRequest()` method. `HeadHTML()` is the usual way to emit the
configured resources and Request key metadata in the page's `<head>` section.
`TailHTML()` is optional; placing it before the closing `</body>` tag applies
updates queued during initial rendering before the WebSocket connects, which can
reduce flicker. Applications that provide equivalent resources and metadata do
not need to call either helper.

When the client has finished loading the document and parsed the
scripts, the JaWS JavaScript will request a WebSocket connection on
`/jaws/*`, with the `*` being the encoded `Request.JawsKey` value.

On receiving the WebSocket HTTP request, decode the key parameter from
the URL and call the JaWS object's `UseRequest()` method to retrieve the
Request created in the first step. Then call its `ServeHTTP()` method to
start up the WebSocket and begin processing JavaScript events and DOM
updates.

### Request lifecycle invariants

Every `NewRequest` returns a distinct `*Request` identity that is never reused
for another connection; only its internal buffers are pooled. While the `Jaws`
instance is open, `NewRequest` creates a pending request owned by it. `UseRequest`
is the only operation that claims that pending request for a WebSocket, and it
also removes the request from the pending set. A claimed Request finishes after
its WebSocket processing exits: its context is canceled, its buffers are released
to the pool, and its key is reserved until the Request is collected rather than
reassigned. Completion leaves the lock-free Element fields and the id counter intact, so if an
early `/jaws/<key>` callback claims and tears down a Request whose initial render
is still in flight, that render may degrade — the elements and tags rendered so far
are forgotten (a later lookup finds nothing, though newly created elements are
tracked again) — but it stays race-safe: no data race, no reused identity, and no
duplicated element id. Maintenance or the per-IP limit can
instead retire a
non-running Request: its context is canceled, its key becomes unclaimable, and it is
excluded from `Pending` and `RequestCount`. Retirement preserves the Request's
identity, Elements and buffers so an initial HTTP handler still holding it can keep
rendering. For registered Requests, a finished key is not assigned to another Request
while the old one remains reachable; no deadline is guaranteed for later reuse. (A
Request created by `NewRequest` after `Jaws.Close` is never registered and installs no
tombstone, so two such post-close calls could receive the same key — harmless, since
neither is claimable.)

`ServeWithTimeout(requestTimeout)` requires an exact multiple of `time.Second`
from `time.Second` through 2,147,483,646 seconds. Other values have unspecified
behavior.

Before `Request.ServeHTTP` begins WebSocket processing, timeout-based Request
retirement is periodic and approximate, not a hard deadline. `NewRequest`, a
successful `UseRequest`, and `ui.RequestWriter.Write` mark activity using
whole-second samples from the epoch established by `jaws.New()`. Retirement is
checked only during maintenance passes, so it is not timed precisely from those
events.

When `Jaws.WebSocketPingInterval` is positive, the same duration is passed
directly as each keepalive ping's timeout on an active WebSocket. Ping timing does
not use those activity samples or the maintenance schedule.

`*Request` values are borrowed lifecycle objects. Do not store them in
application state or pass them to background goroutines; copy the required
application data and retain the Request context instead.

An `*Element` belongs to its owning Request and embeds a pointer to it.
Render-scoped widgets may retain child Elements they create between render and
update calls within that Request lifecycle, as container helpers do, but should
access them only from those calls. Do not let an Element escape the Request
lifecycle or pass it to background work: once the owning Request finishes it is
unregistered, so the Element receives no further broadcasts or updates, though its
fields are left intact and its methods still operate on the finished Request.
Request identities are never reused, so a stale Element can never come to
represent an unrelated connection.

Event targets are fixed when each event is accepted. A live target remains
eligible if it is later removed, whether removal is reported by the browser or
initiated by the server; an Element removed before acceptance is excluded. A
handler may therefore receive a deleted Element, whose render, update, and queue
helpers are no-ops for the rest of the Request.

Dirtying is two-stage: `Request.Dirty` and `Jaws.Dirty` expand and record their
selectors on the `Jaws` instance, then the serving loop distributes ordinary tags
across live Requests and each exact `*Element` only to its owner before scheduling
`JawsUpdate` calls. Broadcast helpers share the serving loop, so start `Serve` or
`ServeWithTimeout` before calling APIs that broadcast, reload, close sessions, or
rely on dirty updates.

Cancellation flows from the request context, the initial HTTP/WebSocket request,
and `Jaws.Close`. Update paths that cannot return errors report them through
`MustLog`, so long-running applications should configure `Jaws.Logger`.
`Jaws.Log` and a configured `MustLog` enqueue `Logger.Error` calls for serial
asynchronous delivery, so those callbacks do not block JaWS processing. Panics
from those callbacks are contained by the dispatcher. `Close` stops accepting
new log entries and lets those already accepted drain without waiting for them;
`Serve` and `ServeWithTimeout` wait for the drain before returning normally after
shutdown. A blocked `Logger.Error` callback delays later entries and that final
drain.

### Configuration lifecycle

Set exported `Jaws` configuration fields immediately after `jaws.New()` and
before exposing handlers, creating Requests, or starting `Serve()` /
`ServeWithTimeout()`. These fields are ordinary Go fields, not synchronized
live configuration knobs.

If you change fields that affect generated page metadata, such as `Debug` or
the resource list passed to `GenerateHeadHTML()`, call `GenerateHeadHTML()`
before rendering new pages so `Request.HeadHTML()` sees the updated data.

`GenerateHeadHTML()` writes markup for common `.js` and `.css` URLs, including a
trailing `@version` when the final extension is otherwise unrecognized, and for
image and font resources. Every successfully parsed URL is passed to
secureheaders automatic CSP inference. A configured `Jaws.Logger` warns once
for each extra URL that is omitted from the final markup or is absolute or
scheme-relative and cannot contribute an explicit policy source. Warning URLs redact
passwords and omit queries and fragments. Applications may load omitted
resources manually. When automatic inference does not match the request
destination, use the explicit policy setup under
[Secure Response Headers](#secure-response-headers). Resource URLs must come
from trusted application configuration because matched scripts are executable
and CSP permissions apply to origins.

### Maintainer checklist

When changing core request, session, broadcast, or WebSocket code, re-check
these invariants before relying on a green build alone:

* Lock order stays `Jaws.mu -> Request.mu -> Session.mu`, with `Request.muQueue`
  and most element/widget/value locks remaining leaves. Container reconciliation's
  state-before-Request edge stays confined to matching and Element creation; core-lock
  holders never invoke container render or update methods.
* Request identities are never reused; only the internal buffers are pooled.
  Completion (after WebSocket serving) releases queued dirt, elements, tags, and
  messages, detaches the session, unregisters the identity, and reserves the key
  with a tombstone until the Request is collected. It clears only live length,
  relying on the shrink paths to zero vacated entries, and must not mutate
  render-visible Element state a still-running initial renderer may read lock-free.
  Non-running retirement instead preserves the Request's Elements and buffers for an
  initial HTTP handler that still holds it.
* Dirty dispatch expands its inputs once. Ordinary keys target registered elements;
  an expanded `*Element` targets that exact Element on its owning Request. Neither
  path lets a queued update reach a finished, unregistered request (whose key stays
  reserved, never reassigned to another Request, while it remains reachable).
* Session grace windows remain deliberate for unclaimed, claimed, failed-upgrade,
  and closed-WebSocket requests.
* WebSocket upgrades keep the single-use key, client-IP binding, and Origin
  host/scheme checks together; changes to trusted forwarded headers must preserve
  the same fail-closed behavior.

Configure `Jaws.Logger` in long-running applications. Initial render errors are
returned to the caller, but update-time paths such as template refreshes and
dynamic child appends cannot return errors to browser event handlers; they are
reported through `MustLog()`, which panics when no logger is configured.

### WebSocket keepalive ping

JaWS can periodically ping active WebSocket connections to detect peers
that disappeared without a close handshake.

Set `Jaws.WebSocketPingInterval` to control this. The default is
`jaws.DefaultWebSocketPingInterval` (1 minute). Set it to `0` or a
negative value to disable keepalive pings.

### Safe to call before `Serve()`

The following APIs are safe to call before starting the JaWS processing
loop (`Serve()` or `ServeWithTimeout()`):

* Construction and lifecycle: `jaws.New()`, `(*Jaws).Close()`, `(*Jaws).Done()`.
* Configuration: `(*Jaws).AddTemplateLookuper()`, `(*Jaws).RemoveTemplateLookuper()`,
  `(*Jaws).LookupTemplate()`, `(*Jaws).GenerateHeadHTML()`, `(*Jaws).Setup()`,
  `(*Jaws).FaviconURL()`.
* Inspection and logging helpers: `(*Jaws).RequestCount()`, `(*Jaws).RequestCounts()`,
  `(*Jaws).Pending()`, `(*Jaws).SessionCount()`, `(*Jaws).Sessions()`,
  `(*Jaws).Log()`, `(*Jaws).MustLog()`.
* Static/ping JaWS endpoints via `(*Jaws).ServeHTTP()`:
  `/jaws/.ping`, `/jaws/.jaws.<hash>.js`, `/jaws/.jaws.<hash>.css`.

Broadcasting APIs are not safe before the processing loop starts. In particular,
`(*Jaws).Broadcast()` (and helpers that call it), `(*Session).Broadcast()`,
`(*Session).Reload()` and `(*Session).Close()` may block before `Serve()` or
`ServeWithTimeout()` is running.

### Secure Response Headers

Use `(*Jaws).SecureHeadersMiddleware(next)` to wrap page handlers with a
security-header baseline and a `Content-Security-Policy` generated from the
resource URLs currently configured for JaWS.

The baseline headers come from
[`github.com/linkdata/secureheaders`](https://github.com/linkdata/secureheaders).

The middleware starts from `secureheaders.DefaultHeaders()`, replaces
`Content-Security-Policy` with `jw.ContentSecurityPolicy()`, and does not trust
forwarded HTTPS headers.

```go
page := ui.Handler(jw, "index", bind.New(&mu, &f))
http.DefaultServeMux.Handle("GET /", jw.SecureHeadersMiddleware(page))
```

When a resource needs an explicit destination, use `secureheaders.Middleware`
instead of the JaWS convenience middleware. The custom policy must include
every external resource the page loads, including resources configured through
`GenerateHeadHTML()`; a zero `Destination` keeps automatic inference. Leave a
context-mismatched resource out of `GenerateHeadHTML()` and load it manually.
Given a parsed, automatically loaded `scriptURL` and a manually fetched
`fetchURL`:

```go
headers := secureheaders.DefaultHeaders()
headers.Set("Content-Security-Policy", secureheaders.BuildContentSecurityPolicy(
	secureheaders.Resource{URL: scriptURL},
	secureheaders.Resource{URL: fetchURL, Destination: secureheaders.ResourceDestinationConnect},
))
http.DefaultServeMux.Handle("GET /", secureheaders.Middleware{Handler: page, Header: headers})
```

### Routing

JaWS doesn't enforce any particular router, but it does require several
endpoints to be registered in whichever router you choose to use. All of
the endpoints start with "/jaws/", and `Jaws.ServeHTTP()` will handle all
of them.

* `/jaws/.jaws.<hash>.css`

  Serves the built-in JaWS stylesheet.

  The response should be cached indefinitely.

* `/jaws/.jaws.<hash>.js`

  Serves the built-in JaWS client-side JavaScript.

  The response should be cached indefinitely.

* `/jaws/[0-9a-v]+` (and `/jaws/[0-9a-v]+/noscript`)

  The WebSocket endpoint, where the path component is the generated lowercase
  base-32 request key. When you register `Jaws.ServeHTTP()` for `GET /jaws/`,
  this is handled automatically. Custom routers that dispatch the endpoint
  themselves should parse the trailing string with
  `key.Parse()` (`github.com/linkdata/jaws/lib/key`) and then retrieve the
  matching JaWS Request with the JaWS object's `UseRequest()` method.

  If the Request is not found, return a **404 Not Found**, otherwise 
  call the Request `ServeHTTP()` method to start the WebSocket and begin
  processing events and updates.

* `/jaws/.tail/<key>`

  Serves the deferred "tail" script for a Request, emitted by `TailHTML()` at the
  end of the page body. The `<key>` identifies the Request; `Jaws.ServeHTTP()`
  looks it up and writes the script. Handled automatically when you register
  `GET /jaws/`.

  The response should not be cached.

* `/jaws/.ping`

  This endpoint is called by the JavaScript while waiting for the server to
  come online. This is done in order to not spam the WebSocket endpoint with
  connection requests, and browsers are better at handling XHR requests failing.

  If you don't have a JaWS object, or if its completion channel is closed (see
  `Jaws.Done()`), return **503 Service Unavailable**. If you're ready to serve
  requests, return **204 No Content**.
  
  The response should not be cached.

Handling the routes with the standard library's `http.DefaultServeMux`:

```go
jw, err := jaws.New()
if err != nil {
  panic(err)
}
defer jw.Close()
go jw.Serve()
http.DefaultServeMux.Handle("GET /jaws/", jw)
```

Handling the routes with [Echo](https://echo.labstack.com/):

```go
jw, err := jaws.New()
if err != nil {
  panic(err)
}
defer jw.Close()
go jw.Serve()
router := echo.New()
router.GET("/jaws/*", func(c echo.Context) error {
  jw.ServeHTTP(c.Response().Writer, c.Request())
  return nil
})
```

### HTML rendering

HTML output elements (e.g. `ui.NewDiv()` and `ui.RequestWriter.Div()`) accept values that can
be made into a `bind.HTMLGetter` using `bind.MakeHTMLGetter()`.

In order of precedence, this can be:
* `bind.HTMLGetter`: `JawsGetHTML(*Element) template.HTML` to be used as-is.
* `bind.Binder[string]` or `bind.Getter[string]`: `JawsGet(*Element) string` that will be escaped using `html.EscapeString`.
* `fmt.Stringer`: `String() string` that will be escaped using `html.EscapeString`.
* a static `template.HTML` or `string` to be used as-is with no HTML escaping.
* everything else is rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.

You can use `bind.New(...).GetHTML(...)`, `bind.HTMLGetterFunc()` or `bind.StringGetterFunc()` to build a custom renderer
for trivial rendering tasks, or define a custom type implementing `HTMLGetter`.
Plain strings are treated as trusted HTML. Escape untrusted string input yourself, or pass it through
a `bind.Getter[string]`, `bind.StringGetterFunc()` or `fmt.Stringer` so JaWS escapes it before rendering.

Initial HTML rendering returns errors directly. Later updates run from the
request processing loop, so custom `JawsUpdate` implementations should keep
their work deterministic and report unrecoverable failures through
`Element.Request.MustLog()` or `Element.Jaws.MustLog()`.

`ui.RequestWriter.Register` is an advanced escape hatch primarily for attaching a
custom, render-independent updater to otherwise static template-authored HTML.
The registered HTML is intended to contain no JaWS widgets. Render standard
widgets through their normal helpers; `Register` makes no compatibility guarantees
for using them as its updater or inside its HTML.

### Data binding

HTML input elements bind browser state to Go values. Text inputs use `string`,
checkable inputs use `bool`, and date inputs use `time.Time`. `ui.Number` and
`ui.Range` accept a `bind.Getter[T]` for every `ui.Numeric` type and become
editable when the source also implements `bind.Setter[T]`. Numeric types include
signed and unsigned integers, `uintptr`, `float32`, `float64`, and named types
with one of those underlying types.

When interaction before the initial WebSocket opens must be prevented, render
native controls disabled or make the interactive region inert. A request
`ConnectFn` can update request-local readiness and dirty the request-specific
readiness tag used by the Template, or the exact Element whose custom updater
removes the gate.

Managed inputs and selects do not support native HTML form reset. A
`<button type="reset">` or `form.reset()` changes browser values without the
per-control input/change events JaWS transports, so it does not update Go state.
Use a JaWS-handled button with `type="button"` that resets the authoritative Go
values and dirties their tags.

Each `ui.Radio` binds one independent boolean. Radios in the same native HTML
group, as determined by their name and form/tree scope, do not become a
server-side group. The browser unchecks peers without sending their input
events, so separately bound values can diverge. Use
`RequestWriter.RadioGroup` with a single-select `named.BoolArray` whose
`named.Bool.Name` values are distinct, or custom setters that clear peers as one
synchronized state change and then dirty every changed binding.

Since all data access needs to be protected with locks, you will usually use
`bind.New()` to create a `bind.Binder[T]` that combines a (RW)Locker and a
pointer to a value of type `T`. It also allows you to add chained setters,
getters, and on-success handlers.

Writable sources used with `ui.NewText`, `ui.NewPassword`, `ui.NewTextarea`,
`ui.NewCheckbox`, `ui.NewRadio`, `ui.NewNumber`, `ui.NewRange`, and `ui.NewDate`
need a stable dirty target derived from the source so dirty-driven updates can
reconcile browser state. Editable Number and Range sources must expand to at
least one stable, usable tag when rendered.
`bind.New(&mu, &value)` exposes the backing pointer. A custom setter can instead
be pointer-valued or implement `JawsGetTag` and return a target that expands to
at least one stable, usable key; `JawsGetTag` takes precedence over the setter's
identity. Tags passed as render parameters register the Element but do not
replace its setter-derived dirty target.

Number and Range retain the bound Go type through parsing, formatting, and setter
calls. Integer inputs use base-10 integer syntax and enforce the bound type's
width. Floating-point inputs enforce their bound type's width; `float32` values
parse and format at 32-bit precision.
A getter-only Number renders `readonly`; a getter-only Range renders `disabled`.
Static numeric values passed to the template helpers use these getter-only forms.

Number sends edits on the browser's `change` event. Pending edits remain
browser-local until then. Range sends live `input` events while its thumb moves.
When an event reaches the widget's own input handler, Number and Range reconcile
accepted and rejected edits with the source's canonical formatting. Both silently
reject browser text that is invalid for the source type and restore the canonical
source value only on the originating control.

Named numeric sources work directly in templates:

```go
type Percent uint8

type Edit struct {
	Percent bind.Binder[Percent]
}
```

```gotemplate
{{$.Number .Dot.Percent `step="1"`}}
{{$.Range .Dot.Percent `min="0"` `max="100"` `step="1"`}}
```

This validator rejects forbidden username characters while accepting ordinary
incremental edits. Its slice makes the setter value non-comparable, so
`JawsGetTag` exposes the backing pointer:

```go
type validatedText struct {
	bind.Setter[string]
	dirtyTag  *string
	forbidden []rune // immutable
}

func (s validatedText) JawsSet(elem *jaws.Element, value string) (err error) {
	for _, r := range value {
		if slices.Contains(s.forbidden, r) {
			err = errors.New("value contains a forbidden character")
			return
		}
	}
	err = s.Setter.JawsSet(elem, value)
	return
}

func (s validatedText) JawsGetTag() any { return s.dirtyTag }

func newUsernameInput(mu *sync.RWMutex, value *string) *ui.Text {
	return ui.NewText(validatedText{
		Setter:    bind.New(mu, value),
		dirtyTag:  value,
		forbidden: []rune{' ', '/', '\\'},
	})
}
```

The reusable nonnumeric input bases cannot automatically reconcile rejected or
normalized browser values without a valid setter-derived target. Number and
Range instead reject an editable source without a usable tag during rendering.
See [`ui.Input`](https://pkg.go.dev/github.com/linkdata/jaws/lib/ui#Input) for the
complete contract.

### Session handling

JaWS has non-persistent session handling integrated. Sessions won't
be persisted across restarts and must have an expiry time.

Use one of these patterns:

* Wrap page handlers with `Jaws.SessionMiddleware(handler)` to create a session when none exists.
* Call `Jaws.NewSession(w, r)` explicitly to create and attach a fresh session cookie.
* Set `Jaws.AutoSession` to lazily create an anonymous session during a
  successful WebSocket upgrade when a Request has none.

Manual session creation is still the primary pattern when the initial render
depends on session data. This is especially important for authentication:
authenticate the user, create or retrieve the session, and populate it before
calling `NewRequest()` and rendering the page.

When subsequent Requests are created with `NewRequest()`, if the
HTTP request has the cookie set and comes from the correct IP,
the new Request will have access to that Session.

Session key-value pairs can be accessed using `Request.Set()` and
`Request.Get()`, or directly using a `Session` object. It's safe to
do this if there is no session; `Get()` will return nil, and `Set()`
will be a no-op.

Sessions are bound to the client IP JaWS sees. Attempting to access an
existing session from a different non-loopback IP will fail. Loopback
addresses are treated as the same client so a reverse proxy connecting to the
backend over loopback does not break binding; in deployments where every
request reaches JaWS from loopback, IP binding is effectively disabled unless
`Jaws.TrustForwardedHeaders` is enabled behind a single trusted reverse proxy.

No data is stored in the client browser except the randomly generated 
session cookie. You can set the cookie name in `Jaws.CookieName`, the
default is derived from the executable name and falls back to `jaws`.

### A note on the Context

The Request object stores a context.Context in one of its fields,
contrary to recommended Go practice.

The reason is that there is no unbroken call chain from the time the Request
object is created when the initial HTTP request comes in and when it is
requested during the JavaScript WebSocket HTTP request.

`Request.SetContext` must return a non-nil context derived from the current
context passed to it. If the returned context is canceled or its deadline
expires, a running Request's WebSocket loop wakes promptly even while idle; no
browser event or broadcast is needed.

Background work that must cancel the Request should retain its own derived
context and cancellation function, not the Request pointer:

```go
var workCtx context.Context
var cancel context.CancelCauseFunc
rq.SetContext(func(parent context.Context) context.Context {
	workCtx, cancel = context.WithCancelCause(parent)
	return workCtx
})
go func() {
	if err := run(workCtx); err != nil {
		cancel(err)
	}
}()
```

`Jaws.BaseContext` and a context installed by `Request.SetContext` own their
cancellation causes. In the example, if `cancel` wins, both
`context.Cause(workCtx)` and `context.Cause(rq.Context())` expose that cause
unchanged; JaWS does not add a `jaws.ErrRequestCancelled` wrapper. When
background failures need stable classification, cancel with an
application-defined sentinel and use `errors.Is(context.Cause(workCtx),
sentinel)`. `ErrRequestCancelled` identifies a non-nil cause supplied when JaWS
cancels a Request.

A custom `Jaws.BaseContext` or context returned from `Request.SetContext` may
implement the optional `AfterFunc(func()) func() bool` method recognized by
`context.AfterFunc`. Its registration and stop hooks must return promptly and
must not synchronously call the same Jaws or Request, or wait for work that does.
Standard-library contexts need no special handling. An interface-only wrapper
can hide optional methods on a custom context:

```go
type contextOnly struct {
	context.Context
}
```

### Security of the WebSocket callback

While the `Jaws` instance is open, each Request gets a non-zero random 64-bit key
not currently in use. This value is written to the HTML output so the JavaScript
can construct the WebSocket callback URL.

While assigned to a Request, its callback key can claim that Request at most
once. JaWS does not assign keys belonging to registered Requests or
still-reachable retired Requests. A retired key may become eligible for reuse
after its Request becomes unreachable, but reuse timing is unspecified.

In addition to this, Requests that are not claimed by a WebSocket call are
retired at regular intervals. By default an unclaimed Request is retired after
10 seconds.

JaWS also limits unclaimed Requests per client IP. The default
`Jaws.MaxPendingRequestsPerIP` value is 100; setting it to zero or a negative
value disables this cap. When the cap is reached, creating a new Request evicts
the oldest idle pending Request from the same IP. If every pending Request was
created or written recently, JaWS evicts the least recently written one so the
configured maximum is never exceeded. The evicted Request's old WebSocket key
can no longer be claimed. The cap uses the same client IP resolver as
request/session binding:
trusted forwarded headers when `Jaws.TrustForwardedHeaders` is enabled,
otherwise the IP parsed from `http.Request.RemoteAddr`.

To guess and hijack a pending WebSocket callback, an attacker must find its key
before the genuine WebSocket claims it or JaWS retires the Request. Finding a
uniformly random 64-bit key takes on the order of 2^63 distinct guesses on
average within that window.

### Authorization and templates (`Auth`)

Templates rendered through `ui.With` receive an `Auth` value exposing `Data()`,
`Email()` and `IsAdmin()`, letting templates gate UI such as
`{{if .Auth.IsAdmin}}`.

You provide the implementation by setting `Jaws.MakeAuth`. **If you leave
`Jaws.MakeAuth` nil, templates receive the built-in `DefaultAuth`, which is
fail-open: `DefaultAuth.IsAdmin()` returns `true` for every visitor** (`Data()`
and `Email()` are fail-safe — nil and empty). A page that gates privileged UI on
`{{if .Auth.IsAdmin}}` will therefore show it to everyone on an instance that
forgot to set `MakeAuth`.

Always set `Jaws.MakeAuth` in production and treat a nil `MakeAuth` as "no
authorization configured", not "deny". As a safety net, when `MakeAuth` is nil and
a `Jaws.Logger` is configured, JaWS logs a one-time warning the first time a
template evaluates `.Auth.IsAdmin`. Note this is lazy: a page that never gates on
`.Auth.IsAdmin` never logs it, so absence of the warning does not prove `MakeAuth`
is set.

### Production hardening checklist

Before serving a JaWS application outside a local development loop, check these
items explicitly:

* Configure `Jaws.Logger`. Update-time errors and API misuse are reported through
  `MustLog()`, which panics when no logger is configured.
* Configure `Jaws.MakeAuth` whenever templates read `.Auth`, especially
  `.Auth.IsAdmin`. Leaving it nil installs the fail-open `DefaultAuth` described
  above.
* Treat plain `string` values passed to HTML-inner widgets as trusted HTML. Route
  user-controlled text through `bind.Getter[string]`, `bind.StringGetterFunc()`,
  `fmt.Stringer` or template escaping before rendering.
* Define the browser-write policy for every `ui.JsVar`. Implement
  `ui.PathSetter` when paths or collection operations must be allow-listed, or
  set `JsVar.ClientCheck` (for example, `ui.JSONSizeCheck[T](limit)`) to validate
  generic writes atomically. Configure an equivalent check and shared locker on
  every binding that exposes the same `Ptr` or reachable mutable backing state;
  server writes and `PathSetter` values bypass `ClientCheck`.
* Enable `Jaws.TrustForwardedHeaders` only behind a single trusted reverse proxy
  that overwrites `X-Forwarded-For`, `X-Real-IP` and `X-Forwarded-Proto`.
* Run the test suite both with and without `-race` before release: `-race`
  exercises the deadlock lock-order detector and JaWS debug-gated checks, while a
  plain `go test` exercises the crash-safe release tag renderer that `-race` and
  `-tags debug` replace. If the race detector is unavailable, use
  `-tags "debug deadlock"` in place of `-race`.

### Testing

Run the test suite both with and without the `-race` flag:

    go test -race ./...
    go test ./...

Race detection sets `deadlock.Debug = true` and `deadlock.Enabled = true` (see
[Dependencies](#dependencies)), which exercises the deadlock lock-order detector
and JaWS debug-gated checks such as the late-handler panic. A `-race` build (like
`-tags debug`) also selects the full-detail tag renderer in
[`lib/tag`](./lib/tag), so the plain `go test` build is what exercises the
crash-safe release renderer used in production. The tag comparability checks in
`lib/tag` run in every build. CI runs both legs. If the race detector is
unavailable, run `go test -tags "debug deadlock" ./...` (the `debug` tag alone
sets `deadlock.Debug` but does not enable the deadlock detector) alongside a
plain `go test ./...`.

### Dependencies

We try to minimize dependencies outside of the standard library.

* Depends on https://github.com/coder/websocket for WebSocket functionality.
* Depends on https://github.com/linkdata/staticserve for serving hashed static assets.
* Depends on https://github.com/linkdata/jq for JSON path get/set on JsVar values.
* Depends on https://github.com/linkdata/secureheaders for `SecureHeadersMiddleware`.
* Depends on https://github.com/linkdata/deadlock for debug-aware locks.

## Learn more

* Browse the [Go package documentation](https://pkg.go.dev/github.com/linkdata/jaws)
  for an API-by-API overview.
* Inspect the [`example_test.go`](./examples/example_test.go) file for
  compile-checked example programs to copy and adapt. They start a blocking HTTP
  server, so they are illustrations rather than examples executed by `go test`.
* Explore the [demo application](https://github.com/linkdata/jawsdemo)
  to see a more complete, heavily commented project structure.
