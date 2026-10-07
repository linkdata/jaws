# Deployment

[Documentation](README.md) · [Runtime](runtime.md) · [Sessions](sessions.md)

Configure the engine before starting its processing loop or exposing HTTP
handlers. The [getting started](getting-started.md) application binds to
localhost; public applications also need the settings below.

## Logging

Assign `jw.Logger = slog.Default()` or another concurrency-safe `jaws.Logger`.
`MustLog` panics without a logger. Render errors return to their caller, while
update-time failures and event errors use logging paths.

Error delivery is asynchronous and serial. Logger callbacks should return
promptly; the serving loop drains accepted entries before returning normally.
The [logging API](../jaws.go) describes queue limits and drop reporting.

Oversized inbound WebSocket messages close the connection and log the read-limit
error. Other transport errors ordinarily end the Request without a log report;
`Debug` retains their underlying details for logging. Configure `Debug` before
serving and regenerate head markup with `GenerateHeadHTML` when changing it.

## Authorization

Set `Jaws.MakeAuth` when template output uses authorization. A nil `MakeAuth`
uses `DefaultAuth`, whose `IsAdmin` returns true for every visitor. With a logger,
the first evaluation warns about that default; absence of a warning does not
establish that authorization is configured.

Supply an application implementation of `jaws.Auth` (`Data`, `Email`, and
`IsAdmin`). For example, `authForSession` below is the application's authenticated
session lookup and must return an authorization value even for anonymous users:

```go
jw.MakeAuth = func(rq *jaws.Request) jaws.Auth {
    return authForSession(rq.Session())
}
```

Templates read the result through `$.Auth`, for example
`{{if $.Auth.IsAdmin}}...{{end}}`. See the [Auth API](../contracts.go).

Template visibility is not permission enforcement. Validate authorization in
event handlers and other operations that read or mutate protected data. A handler
can use the same application lookup with `elem.Request.Session()`. A
WebSocket connection establishes a client connection, not affirmative user intent.
Use a semantic action such as a button click when that distinction matters.

## HTML and browser data

Plain strings passed to HTML-producing JaWS helpers are trusted raw HTML.
For untrusted text, use `bind.StringGetterFunc`, an escaping Binder, or another
escaping adapter described in [bindings](bindings.md). Attribute parameters and
`template.HTMLAttr` are trusted syntax; build values with `htmlio.Attr` and a
trusted attribute name. See [HTML utilities](utilities.md).

A browser-writable `ui.JsVarStore` needs a `ClientCheck` that validates the
complete tentative value and canonical path under the store lock. A nil check
denies browser writes. Use one store for bindings of the same shared value and
avoid shared mutable aliases within trees that receive partial patches.
[JavaScript values](ui/jsvar.md#bind-javascript-variables) explains the contract.

Each inbound WebSocket message payload is limited to 32 KiB, including JaWS
protocol and JSON overhead. Standard
widgets do not chunk payloads. Use HTTP endpoints for uploads or large data.
[Transport](transport.md) describes record sizes and limits.

## Response headers and resources

Wrap page handlers in `jw.SecureHeadersMiddleware(page)` to apply the
`secureheaders` baseline with JaWS's generated Content-Security-Policy.
Behind a proxy that terminates TLS, set HSTS at the proxy: this middleware
does not trust forwarded HTTPS headers, even when `TrustForwardedHeaders` is
enabled for JaWS's other checks.

`NewRequest` makes rendered pages non-cacheable. Keep callback and tail-script
responses non-cacheable; hashed static assets may be cached.

`GenerateHeadHTML` generates resource markup and infers CSP destinations from
configured URLs. Resource URLs are trusted configuration: scripts can execute
and origins affect the policy. Use an explicit `secureheaders.Middleware`
policy when automatic inference does not represent the required destination,
and load those resources with the corresponding markup. See
[Bootstrap](bootstrap.md) and [browser resources](browser.md).

## Reverse proxies

Enable `TrustForwardedHeaders` only behind one controlled reverse proxy that
removes client-supplied forwarding headers and sets the client IP and scheme.
Sanitize all recognized headers:

- `X-Forwarded-For` and `X-Real-IP` for client IP binding.
- `X-Forwarded-Proto`, `X-Forwarded-Ssl`, `Front-End-Https`, and `Forwarded` for scheme resolution.

JaWS parses only the rightmost comma-separated element of each header's last
line. It prefers a valid `X-Forwarded-For` address, falling back to `X-Real-IP`.
If neither parses or both valid addresses disagree, it uses the transport peer.
Set or remove both IP headers. A TLS-terminating proxy forwarding plain HTTP must supply the trusted
scheme, or HTTPS-page upgrades fail with `ErrWebsocketOriginWrongScheme`.

Preserve the page host, `/jaws/` paths, and WebSocket upgrades. JaWS checks the
Origin scheme and host against the initial request, binds the callback to the
client IP, and permits each request key to be claimed once.

## Capacity

`MaxEventRate` limits incoming `Click`, `ContextMenu`, `Input`, and `JsVar`
records together to 100 per second per Request by default. Zero selects the
default; a negative value disables the limit. Records are paced in order without
being dropped, with an initial 10 ms wait at the default rate. `Remove` consumes
no event budget but cannot overtake a waiting event. Fast range-slider or
JavaScript activity can accumulate delay; configure the rate for the
application's workload.

The per-Request limit does not bound aggregate broadcasts from many clients.
Broadcast helpers such as `Append`, `SetInner`, and `Alert` send a command per
call. On shared tags, each client's events can increase every subscriber's
required bandwidth. Requests whose queues fill are disconnected with
`ErrRequestOverloaded`. Prefer `Dirty`/`Update` coalescing for shared state;
per-event broadcast byte rates must fit the slowest supported client link.

`MaxPendingRequestsPerIP` defaults to 100. IPv4 clients share a bucket by
address; IPv6 uses a /64, with the well-known `64:ff9b::/96` NAT64 prefix mapped
to its embedded IPv4 address. At capacity a new page retires the oldest idle
pending Request, or the least recently written one if all are fresh. Claimed
and active Requests are not evicted by this cap. A non-positive limit disables it.

Clients behind the same NAT, IPv6 /64, or unconfigured proxy share that bucket.
Their page loads can retire each other's pending callback keys. Set limits for
expected page traffic and render-to-connect time; rate-limit public page and
Session-creating routes at the proxy when needed. The pending cap does not bound
active connections. [Session limits](sessions.md#lifetime-and-limits) separately
bound registered Sessions.

Timeout retirement is periodic. Request creation, writes, and claims record
activity at whole-second resolution; the default 10-second timeout is not an
exact deadline. Configure HTTP server and proxy limits as well as JaWS limits
for the application workload.
