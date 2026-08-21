# AI guidance for github.com/linkdata/jaws

[Back to the human overview and quick start](./README.md).

This is the version-matched implementation guide for the root `jaws` package.
Use exported symbol documentation as the authority for supported caller-facing
behavior. Use this file for package-wide ownership, lifecycle, security, and
maintenance invariants, and follow the package-local guide when work belongs to
a subpackage.

## Package guide index

The module contains 16 Go packages, each with one package-local guide:

* [`github.com/linkdata/jaws`](./AI.md) -- core requests, sessions, serving,
  routing, security, and lifecycle.
* [`github.com/linkdata/jaws/examples`](./examples/AI.md) -- canonical setup and
  compile-checked examples.
* [`github.com/linkdata/jaws/examples/minesweeper`](./examples/minesweeper/AI.md)
  -- example domain state and targeted dirtying.
* [`github.com/linkdata/jaws/jawsboot`](./jawsboot/AI.md) -- embedded Bootstrap
  assets and version updates.
* [`github.com/linkdata/jaws/jawstest`](./jawstest/AI.md) -- isolated request
  harnesses.
* [`github.com/linkdata/jaws/lib/assets`](./lib/assets/AI.md) -- bundled browser
  resources and the DOM trust boundary.
* [`github.com/linkdata/jaws/lib/bind`](./lib/bind/AI.md) -- binders, adapters,
  conversion, locking, and setter targets.
* [`github.com/linkdata/jaws/lib/htmlio`](./lib/htmlio/AI.md) -- low-level trusted
  HTML emission.
* [`github.com/linkdata/jaws/lib/jid`](./lib/jid/AI.md) -- request-scoped element
  identifiers.
* [`github.com/linkdata/jaws/lib/key`](./lib/key/AI.md) -- request-key encoding
  and parsing.
* [`github.com/linkdata/jaws/lib/named`](./lib/named/AI.md) -- named inputs and
  single-select collections.
* [`github.com/linkdata/jaws/lib/tag`](./lib/tag/AI.md) -- tag expansion,
  registration, targeting, and rendering.
* [`github.com/linkdata/jaws/lib/templatereloader`](./lib/templatereloader/AI.md)
  -- debug template reloading and last-good retention.
* [`github.com/linkdata/jaws/lib/ui`](./lib/ui/AI.md) -- templates, containers,
  inputs, browser interaction, and standard widgets.
* [`github.com/linkdata/jaws/lib/what`](./lib/what/AI.md) -- command and event
  meanings.
* [`github.com/linkdata/jaws/lib/wire`](./lib/wire/AI.md) -- WebSocket record
  framing and transport limits.

## Module conventions

Throughout this module, nil is unsupported for pointer receivers and required
operational collaborators such as callbacks, handlers, providers, lockers,
writers, file systems, contexts, and pointers to mutable values unless the API
documents a meaning for nil. Unsupported nil use is caller error and may panic
immediately or when the value is used. Nil slices, maps, data values, and results
otherwise follow ordinary Go semantics and the relevant API. An interface that
contains a typed nil is non-nil; its behavior follows its concrete type and the
API receiving it.

Preserve documented zero-value behavior. In particular, the zero `Jaws` value is
not ready for use; construct it with `jaws.New`. Do not add repetitive non-nil
preconditions to individual symbols when the module convention already applies.

## UI and Element ownership

Every non-nil `UI` value must be comparable at runtime and equal to itself. A
value with an interface field holding a slice, map, or function can be statically
comparable while failing at runtime; a value containing `NaN` is comparable but
not reflexive. Container widgets cancel the `Request` when a child violates this
rule, and debug builds assert it in `Request.NewElement`. Keep slice-, map-, and
function-bearing application state behind stable pointers.

UI values and Elements follow these ownership rules:

* Construct fresh UI values for every Request. A UI value may refer to shared
  application state, binders, handlers, and tags when those values are
  synchronized as required.
* Within one Request, a UI value normally backs one live Element. Reuse it for
  multiple live Elements only when the concrete type documents that support and
  does not retain shared Element-specific state.
* A nil `UI` interface passed to `Request.NewElement` is a render/update no-op.
  A typed nil is dispatched normally, so nil-receiver behavior belongs to the
  concrete type's contract.
* An Element belongs to its Request for its entire lifetime. Render-scoped
  widgets may retain child Elements between render and update calls, but neither
  Requests nor Elements belong in application state or background goroutines.
* A registered Element is the unit that receives events and DOM updates. Each
  dynamically managed child must render one addressable direct DOM node carrying
  its Jid-based HTML ID.

Handler registration finishes before event dispatch. `Element.AddHandlers`,
`ApplyParams`, and `ApplyGetter` are render/registration operations. Once
`Element.JawsRender` returns or `Element.Freeze` runs, later handler additions are
dropped and debug builds panic. Incoming events try attached handlers in reverse
registration order and then the Element's UI value; a handler returns
`ErrEventUnhandled` to continue dispatch.

Element update methods queue browser changes during render or update processing.
Pass an Element to `Dirty` to schedule that exact control on its owning Request.
Pass a dependency tag when every registered Element for application state must
update. Both `Request.Dirty` and `Jaws.Dirty` expand through the same global
dispatcher; ordinary tags are not restricted to the Request on which
`Request.Dirty` was called. See the [`tag` guide](./lib/tag/AI.md) for selection
and registration rules and the [`ui` guide](./lib/ui/AI.md) for widget-specific
identity and multiplicity.

## HTTP and WebSocket flow

The normal page flow has two related HTTP requests:

1. A page handler creates a Request with `Jaws.NewRequest`. `HeadHTML` normally
   emits the configured resources and request-key metadata. `TailHTML` is
   optional; placing it before `</body>` applies queued initial updates before
   the WebSocket connects and can reduce flicker.
2. The bundled script connects to `/jaws/<key>`. `Jaws.ServeHTTP` decodes the
   key, claims the pending Request through `UseRequest`, upgrades the connection,
   and begins event and DOM-update processing.

Page responses containing `HeadHTML` or equivalent Request-key metadata must
include the `no-store` Cache-Control directive. An HTTP-cached copy would repeat
the consumed one-use capability and cannot establish another WebSocket
connection. `ui.Handler` sets `Cache-Control: no-store` automatically; custom
page handlers must set it explicitly. A bfcache restoration is handled
separately by the bundled client's `pageshow` reload.

Applications that emit equivalent resources and metadata need not call
`HeadHTML` or `TailHTML`. Custom routers may parse the trailing key with
`key.Parse`, call `UseRequest`, return 404 when it is absent, and then call the
claimed Request's `ServeHTTP` method.

## Request lifecycle and serving

Every `NewRequest` returns a distinct `*Request` identity. Requests are never
pooled or reused; only internal buffers are pooled. While the Jaws instance is
open, a new Request is pending and owned by that instance. `UseRequest` is the
only operation that claims it for a WebSocket, and claiming removes it from the
pending set.

A claimed Request finishes after WebSocket processing exits. Its context is
canceled, pooled buffers are released, live registries are cleared, and its key
remains reserved while the Request is reachable. Completion leaves lock-free
Element fields and the ID counter intact. If an early callback claims and tears
down a Request while its initial renderer is still running, already-rendered
elements and tags may be forgotten, but the renderer remains race-safe and IDs
are not reused or duplicated.

Maintenance or the per-IP cap may instead retire a non-running Request. Its
context is canceled, its key becomes unclaimable, and it leaves `Pending` and
`RequestCount`, while its identity, Elements, and buffers remain available to an
initial HTTP handler that still holds it. A Request created after `Jaws.Close` is
never registered or claimable and installs no key tombstone.

Request timeout behavior is deliberately bounded:

* `ServeWithTimeout` requires an exact multiple of one second from one second
  through 2,147,483,646 seconds. Other values have unspecified behavior.
* Before WebSocket serving begins, retirement is periodic and approximate rather
  than a hard deadline. Request creation, successful claim, and page writes mark
  activity with whole-second samples; maintenance passes decide retirement.
* On an active WebSocket, the timeout bounds each keepalive ping and outbound
  write.

Event targets are fixed when an event is accepted. A target removed after
acceptance may still receive that event; a target removed before acceptance does
not. A handler can therefore receive a deleted Element, whose render, update,
and queue helpers remain no-ops for the rest of the Request.

Dirtying is two-stage: calls expand and record selectors on the Jaws instance,
then the serving loop distributes ordinary tags across live Requests and exact
Elements only to their owners. Broadcasts, session reload/close helpers, and
dirty updates share the serving loop. Start `Serve` or `ServeWithTimeout` before
using them.

### Calls before Serve

The following operations are safe before the processing loop starts:

* Construction and lifecycle: `New`, `Close`, and `Done`.
* Configuration and templates: `AddTemplateLookuper`,
  `RemoveTemplateLookuper`, `LookupTemplate`, `GenerateHeadHTML`, `Setup`, and
  `FaviconURL`.
* Inspection and logging: `RequestCount`, `RequestCounts`, `Pending`,
  `SessionCount`, `Sessions`, `Log`, and `MustLog`.
* Static and ping endpoints through `ServeHTTP`: `/jaws/.ping`, the hashed
  built-in JavaScript URL, and the hashed built-in stylesheet URL.

Request creation and initial page rendering are not supported before the
processing loop starts. Start `Serve` or `ServeWithTimeout` before exposing any
handler that can call `NewRequest`; this also keeps `RequestWriter` and
`Request.MarkWritten` within their supported lifecycle. During defect review,
treat pre-Serve request creation, rendering, activity accounting, and pending
request eviction as caller lifecycle misuse rather than library behavior.

`Broadcast`, `Session.Broadcast`, `Session.Reload`, and `Session.Close` may block
before the processing loop starts.

### Keepalive pings

JaWS pings read-idle WebSockets to detect peers that disappear without a close
handshake. Incoming data and successful pings restart the idle interval; time
spent parsing or delivering data already read does not count toward it.
`WebSocketPingInterval` defaults to `DefaultWebSocketPingInterval` and must be
positive. A non-positive value does not disable probing.

## Context and cancellation

The Request stores a context because page creation and the later WebSocket
callback do not share an unbroken call chain. `Request.SetContext` transforms
the current Request context. Return a context derived from the callback's parent
so deadlines and cancellation continue to propagate. If it is canceled, an idle
WebSocket loop wakes without waiting for a browser event or broadcast.

Background work that must cancel a Request retains its own derived context and
cancellation function, not the Request pointer:

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
cancellation causes. If an independent cancellation or deadline wins, the
Request context exposes that cause directly; JaWS does not wrap it in
`ErrRequestCancelled`. Use an application sentinel when callers need to classify
background failure with `errors.Is`. `ErrRequestCancelled` identifies a non-nil
cause supplied when JaWS cancels a Request.

A custom base or Request context may implement the optional
`AfterFunc(func()) func() bool` method recognized by `context.AfterFunc`. Its
registration and stop hooks must return promptly and must not synchronously call
the same Jaws or Request, or wait for work that does. Standard-library contexts
need no special handling. An interface-only `struct{ context.Context }` wrapper
can hide optional methods.

## Sessions

Sessions are server-side, non-persistent, expiring, and bound to the client IP
seen by JaWS. The browser stores only the random session cookie. Use one of these
creation patterns:

* Wrap a page handler with `Jaws.SessionMiddleware`.
* Call `Jaws.NewSession` explicitly and attach the new cookie.
* Enable `Jaws.AutoSession` to create an anonymous session during a successful
  WebSocket upgrade when the Request has none.

Create or retrieve the session before `NewRequest` when initial rendering or
authentication depends on it. Later Requests with the same valid cookie and IP
can access the Session. `Request.Get` returns nil and `Request.Set` is a no-op
when no Session exists. `Jaws.Close` invalidates every Session, clears its data,
and prevents new Session creation.

Loopback addresses are treated as the same client so a loopback reverse proxy
does not break binding. If all traffic reaches JaWS from loopback, binding is
effectively disabled unless trusted forwarding is configured behind a single
controlled proxy. `CookieName` must be a valid non-empty HTTP cookie name; its
default derives from the executable and falls back to `jaws`.

## Configuration and logging

Set all exported `Jaws` configuration fields immediately after `New` and before
exposing handlers, creating Requests, or starting a serve loop. They are ordinary
fields, not synchronized live settings. If `Debug` or the resource list changes,
call `GenerateHeadHTML` before rendering more pages.

`GenerateHeadHTML` emits common JavaScript and CSS URLs, recognizes image and
font resources, and passes parsed URLs to automatic Content-Security-Policy
inference. A configured logger warns once for resources omitted from markup or
for absolute and scheme-relative URLs whose policy source cannot be represented.
Warnings redact passwords and omit queries and fragments. Resource URLs are
trusted configuration: scripts are executable and their origins affect policy.
Use an explicit `secureheaders` policy when automatic destination inference is
not appropriate.

Configure `Jaws.Logger` for long-running applications. Initial render failures
return to the caller, but update-time paths cannot return errors to browser event
handlers and report them through `MustLog`, which panics without a logger.
An oversized inbound WebSocket message fails with a read-limit error retained in
the Request cancellation cause, which is passed to `Jaws.Log`. Other WebSocket
transport failures ordinarily end the Request as an ordinary cancellation and
are not sent to the logger. When `Jaws.Debug` is enabled, their underlying
transport error is retained in the Request cancellation cause, which is passed
to `Jaws.Log`.
`Jaws.Log` and configured `MustLog` calls enqueue `Logger.Error` callbacks for
serial asynchronous delivery. Callback panics are contained. `Close` stops
accepting new log entries and lets accepted entries drain without waiting;
`Serve` and `ServeWithTimeout` wait for the drain before returning normally. A
blocked logger callback delays later entries and the final drain.

## Routing

Register `Jaws.ServeHTTP` for the `/jaws/` prefix. It owns these routes:

* `/jaws/.jaws.<hash>.css` -- built-in stylesheet; cache indefinitely.
* `/jaws/.jaws.<hash>.js` -- built-in client; cache indefinitely.
* `/jaws/<key>` and `/jaws/<key>/noscript` -- single-use Request callback. The
  key must parse to a nonzero value through `key.Parse`; parsing is
  case-insensitive, while generated URLs use canonical lowercase base 32. A
  missing Request is a 404. See the [`key` guide](./lib/key/AI.md).
* `/jaws/.tail/<key>` -- deferred initial-update script emitted by `TailHTML`;
  do not cache.
* `/jaws/.ping` -- readiness probe used before WebSocket reconnect attempts.
  Return 204 while ready and 503 without a live Jaws instance; do not cache.

The standard library setup is:

```go
jw, err := jaws.New()
if err != nil {
	panic(err)
}
defer jw.Close()
go jw.Serve()
http.DefaultServeMux.Handle("GET /jaws/", jw)
```

JaWS does not require a particular router; other routers must preserve the same
path prefix, status codes, caching behavior, and single-use request claim.

## Security

### Response headers

`Jaws.SecureHeadersMiddleware` applies the
[`secureheaders.DefaultHeaders`](https://pkg.go.dev/github.com/linkdata/secureheaders#DefaultHeaders)
baseline and replaces its Content-Security-Policy with
`Jaws.ContentSecurityPolicy`. It does not itself trust forwarded HTTPS headers.

```go
page := ui.Handler(jw, "index", bind.New(&mu, &value))
http.DefaultServeMux.Handle("GET /", jw.SecureHeadersMiddleware(page))
```

When a resource needs an explicit CSP destination, build a custom
`secureheaders.Middleware` policy that includes every external resource loaded
by the page. Leave a context-mismatched resource out of `GenerateHeadHTML` and
load it manually with its explicit destination.

### WebSocket callback keys

Each open Jaws instance assigns a pending Request a non-zero random 64-bit key
that is not currently in use. A key can claim its Request once. Keys for
registered or still-reachable retired Requests are not reassigned; reuse after a
retired Request becomes unreachable has no timing guarantee.

Unclaimed Requests retire periodically, after 10 seconds by default. JaWS also
limits them per client IP. `MaxPendingRequestsPerIP` defaults to 100; a
non-positive value disables the cap. At the cap, a new Request evicts the oldest
idle pending Request for that IP, or the least recently written one when all are
fresh. The evicted key cannot be claimed.

Guessing a uniformly random pending key takes about 2^63 distinct guesses on
average, and the attacker must succeed before the real browser claims the key or
the Request retires.

WebSocket upgrades keep the single-use key, client-IP binding, and Origin host
and scheme checks together. Do not weaken one while changing another.

### Trusted proxy headers

Enable `TrustForwardedHeaders` only behind one controlled reverse proxy. The
proxy must remove client-supplied forwarding headers and set the client IP and
scheme itself. JaWS uses `X-Forwarded-For` and `X-Real-IP` for binding. Scheme
resolution recognizes `X-Forwarded-Proto`, `X-Forwarded-Ssl`, `Front-End-Https`,
and `Forwarded`, so the proxy must sanitize all of them. Without trusted scheme
forwarding, a TLS-terminating proxy that talks plain HTTP to JaWS causes
HTTPS-page WebSocket upgrades to fail with `ErrWebsocketOriginWrongScheme`.

### Authorization

Templates rendered through `ui.With` receive an `Auth` value. `Jaws.MakeAuth`
provides it. A nil `MakeAuth` installs the built-in fail-open `DefaultAuth`, whose
`IsAdmin` method returns true for every visitor. Set `MakeAuth` whenever template
output depends on authorization.

With a logger configured, JaWS warns once when a template first evaluates
`.Auth.IsAdmin` while `MakeAuth` is nil. The warning is lazy: its absence does
not prove authorization is configured.

## Locking and maintainer invariants

When changing request, session, broadcast, or WebSocket code, preserve these
cross-file invariants:

* Core lock order is `Jaws.mu -> Request.mu -> Session.mu`.
  `Request.muQueue` and most Element, widget, and application-value locks remain
  leaves.
* Container reconciliation's state-before-Request edge is confined to matching
  and Element creation. Core-lock holders never call container render or update
  methods.
* Request identities are never reused. Completion releases queued dirt,
  Elements, tags, messages, and Session attachment, unregisters the identity,
  and reserves the key while the Request remains reachable.
* Completion clears live lengths and relies on shrink paths to zero vacated
  entries. It must not mutate render-visible Element state that a concurrent
  initial renderer may read without a lock. Non-running retirement preserves
  Elements and buffers for that renderer.
* Dirty inputs expand once. Ordinary keys target registered Elements and an
  expanded `*Element` targets only itself on its owning Request. Neither path
  may update a finished unregistered Request.
* Session grace windows remain deliberate for pending, claimed,
  failed-upgrade, and closed-WebSocket Requests.
* Upgrade changes preserve single-use keys, IP binding, and fail-closed Origin
  validation together.

Review changes for correctness and performance, not just compilation. A merge
that selects individually valid files can still violate cross-file assumptions.

## Production hardening

Before exposing an application outside local development:

* Configure `Jaws.Logger` so update-time failures are reported instead of
  panicking through `MustLog`.
* Configure `Jaws.MakeAuth` whenever templates use authorization; the default is
  fail-open.
* Treat plain strings passed to HTML-producing widgets as trusted HTML. Route
  untrusted text through escaping conversion; see the [`bind`](./lib/bind/AI.md)
  and [`ui`](./lib/ui/AI.md) guides.
* Define the browser-write policy for every `ui.JsVar`. Use `ui.PathSetter` for
  path allow-lists or a `ClientCheck` for atomic generic-write validation, and
  apply equivalent protection and the same lock to every binding that exposes
  shared mutable state. See the [`ui` guide](./lib/ui/AI.md).
* Keep browser-to-server messages below the transport limit and use HTTP uploads
  for large data. An oversized inbound message closes the Request connection
  rather than rejecting only one value, and its read-limit error is reported
  through `Jaws.Log`; see the [`wire` guide](./lib/wire/AI.md).
* Use `SecureHeadersMiddleware` or an equivalent explicit security-header
  policy.
* Configure trusted forwarding only behind a proxy that sanitizes every
  recognized forwarding header.
* Run both race/debug and production-build test legs before release.

## Repository verification matrix

Run commands from the module root. Start with focused tests for the changed
package, then use the complete gate for broader changes:

```bash
go generate ./...
go vet ./...
gofmt -l .
staticcheck ./...
golangci-lint run
gosec ./...
go build ./...
JAWS_REQUIRE_NODE=1 go test -race ./...
JAWS_REQUIRE_NODE=1 go test ./...
```

Generation should leave the intended tracked files unchanged unless the change
deliberately updates generated assets. The race leg enables deadlock detection
and debug-gated checks. It also selects the detailed tag renderer; the plain
test leg exercises the crash-safe release renderer used in production. See the
[`tag` guide](./lib/tag/AI.md) for the build-mode details.

`JAWS_REQUIRE_NODE=1` turns a missing Node runtime into a failure instead of
silently skipping the browser-client behavior tests. On a Linux host capable of
executing 386 binaries, also run the 32-bit numeric leg used by CI:

```bash
GOARCH=386 CGO_ENABLED=0 go test ./lib/bind/... ./lib/ui/...
```

That leg exercises word-size-dependent `int` and `uint` bounds. On other hosts,
require the `build-386` CI job to pass. If the race detector is unavailable, run
`JAWS_REQUIRE_NODE=1 go test -tags "debug deadlock" ./...` plus
`JAWS_REQUIRE_NODE=1 go test ./...`.

For performance work, commit a benchmark that exercises the changed path, use
`b.RunParallel` for contention changes or `b.ReportAllocs` for per-operation
work, and compare at least six runs of before and after results with `benchstat`.
