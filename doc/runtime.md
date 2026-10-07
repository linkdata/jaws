# Runtime and requests

[Documentation](README.md) · [Pages and widgets](ui/README.md) · [Sessions](sessions.md)

## Configure and start the engine

Construct the engine with `jaws.New`; the zero `Jaws` value is not ready for use.
Set configuration fields, register templates, and configure resources before
starting handlers or the processing loop. `StatusMetrics` is atomic and can
change while serving; other exported configuration fields are ordinary fields.

Start one `go jw.Serve()` before exposing page handlers. `Serve` distributes
updates and broadcasts and performs maintenance. `Serve` uses a 10-second
timeout; `ServeWithTimeout` accepts a positive whole-second duration within its
[documented range](../serve.go). This timeout governs periodic retirement of inactive Requests before
WebSocket processing and bounds each active WebSocket ping and outbound write.
It is not a maximum connection lifetime or an exact page-render deadline.

`jw.Close()` begins shutdown, cancels Requests, invalidates Sessions, and stops
new Session creation. Active WebSocket handlers finish asynchronously. The
serving loop drains accepted error logs before returning; a blocked logger can
delay that return.

## Follow a page request

1. `ui.Handler` calls `NewRequest` before writing headers and sets
   `Cache-Control: no-store`. It renders the page without a generated wrapper.
2. Rendering creates Elements, associates dependency tags, and installs event
   handlers. `HeadHTML` supplies resources and a single-use callback key.
3. The browser connects to `/jaws/<key>`. `UseRequest` claims the pending Request;
   the Request validates the WebSocket upgrade and starts processing events.
4. Event handlers mutate application state and dirty dependency tags. The engine
   selects matching Elements and their widgets queue DOM updates.
5. WebSocket completion cancels the Request and releases its live registries.
   An unclaimed Request can instead retire through maintenance or the pending cap.

Every Request has a distinct identity. An initial renderer may still hold a
Request after retirement, but the key cannot be claimed. Retain synchronized
application state and contexts in background work; do not retain Requests or
Elements outside the lifecycle that supplied them.

`ui.Handler` reuses its configured dot across HTTP requests, so the dot must
support concurrent rendering. Construct a handler inside an outer HTTP handler
when the dot depends on the user or request; see [sessions](sessions.md).
The page Element does not update itself. Nested widgets own live updates.

For custom rendering, call `NewRequest(w, r)` before writing a response and use
`ui.RequestWriter{Request: rq, Writer: w}` to record render activity. Low-level writers call
`rq.MarkWritten()` before each initial HTML write. `HeadHTML` alone does not
set response headers.

## Connect and handle events

A top-level page dot implementing `jaws.ConnectHandler` supplies `JawsConnect`.
`ui.Handler` installs it before template execution; an accepted WebSocket invokes
it, not the page GET. Promoted methods count. `JawsConnect` implemented only by a
nested Template dot is not installed. Click, context-menu, and input handlers
attach to rendered widgets or nested JaWS Templates; defining them only on the
page dot does not install them. See `ConnectFn` in
[request.go](../request.go) for callback ordering and permitted operations.

Register event handlers during rendering, before the Element is frozen.
Dispatch tries attached handlers in reverse registration order, then the
Element's UI. Return `jaws.ErrEventUnhandled` to try the next handler. Other
errors are logged and, when possible, displayed as browser danger alerts.
JavaScript store rejections are logged and corrected without automatic alerts;
the application's `ClientCheck` can send its own [feedback](ui/jsvar.md).
Recovered panics are also logged without automatic alerts. Errors matching
`jaws.ErrEventLogOnly` suppress the automatic alert, including wrapped or joined
errors; explicit `Request.Alert` calls are unaffected.
If no handler accepts an event, it is ignored without an error or alert.

Browser event handlers run serially within each Request, concurrently with update
processing and handlers in other Requests. Synchronize shared application state
and return promptly.

Queued browser commands wake the Request processing loop. Maintenance also wakes
Requests with queued output. State-driven rendering still uses `Dirty` and the
normal update interval.

An event accepted before its target is removed can still reach that deleted
Element. Its render, update, and queue helpers become no-ops. Browser events
while disconnected are not replayed; see [browser integration](browser.md).

Call `jw.Dirty(tag)` after changing shared state. `rq.Dirty(tag)` has the same
cross-request scope for ordinary tags. Passing an Element targets just that
Element in its owning Request. Dirtying schedules work; it does not synchronously
render or acknowledge a browser update. See [tags and updates](tags.md).

## Route HTTP requests

Register `mux.Handle("GET /jaws/", jw)` alongside page routes.

| Path | Purpose |
| --- | --- |
| `/jaws/.jaws.<hash>.js`, `/jaws/.jaws.<hash>.css` | Bundled, publicly cacheable assets |
| `/jaws/<key>` | Single-use WebSocket callback |
| `/jaws/<key>/noscript` | Callback when scripting is unavailable |
| `/jaws/.tail/<key>` | Deferred initial-update script; not cacheable |
| `/jaws/.ping` | Reconnect readiness probe; 204 while open, 503 after shutdown; not cacheable |

Custom routers must preserve the prefix, status codes, and cache behavior.
Custom callback routing can use `key.Parse`, `jw.UseRequest`, then
`rq.ServeHTTP`; return 404 if no Request is claimed. Details are in
[transport](transport.md).

Read-idle WebSockets are probed at `WebSocketPingInterval` (one minute by
default). Incoming data and successful pings restart the interval. The configured
interval must be positive; zero does not disable pings.

## Cancel background work

`Request.Context()` can outlive the callback that supplies the Request. Derive
background work from it. To let a background failure end the Request, install a
derived context in `JawsConnect`, or in a custom page handler after `NewRequest`:

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

Here `run` is application work that observes `workCtx.Done()`. The transform
runs under the Request lock: return a context derived from its argument and do
not block or call back into the Request. Cancellation wakes an idle connection.

Cancellation from `BaseContext` or an installed context preserves its cause.
A non-nil cause supplied to JaWS cancellation is wrapped with
`ErrRequestCancelled`. Use `context.Cause` and `errors.Is` to classify failures.

## Display status

Enable automatic status-tag updates with
`jw.StatusMetrics.Store(jaws.StatusMetricAll)`, or select individual flags.
Give a getter the matching stable tag, such as `PendingRequestCountTag()` for
`jw.Pending()` or `ErrorCountTag()` for `jw.ErrorCount()`.

`RequestCounts` returns total and active Request counts; the total includes
pending, claimed, and active Requests. `Pending` counts those waiting to be
claimed. Tabs count separately. `ActiveSessionCount` counts each Session with an
active Request once; `SessionCount` also includes retained and expired Sessions
awaiting cleanup. Error counting continues without a logger and after shutdown.

See [status.go](../status.go) for metric flags and tag accessors and
[deployment](deployment.md) for logging and resource limits.
