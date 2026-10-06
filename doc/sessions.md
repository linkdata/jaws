# Sessions

[Documentation](README.md) · [Runtime](runtime.md) · [Deployment](deployment.md)

Sessions hold server-side, non-persistent data associated with a random browser
cookie and the client IP seen by JaWS. They expire and do not survive an engine
restart. A Session identifies stored state; application authentication and
authorization remain separate concerns.

## Create a session before rendering

Wrap the page in `SessionMiddleware` when initial rendering needs a Session.
This gives each Session its own range value using the
[getting-started template](getting-started.md):

```go
var sessionInit sync.Mutex
page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    session := jw.GetSession(r)
    sessionInit.Lock()
    value := session.Get("percent")
    if value == nil {
        percent := 50
        value = bind.New(new(sync.Mutex), &percent)
        session.Set("percent", value)
    }
    sessionInit.Unlock()
    ui.Handler(jw, "index", value).ServeHTTP(w, r)
})
mux.Handle("GET /{$}", jw.SecureHeadersMiddleware(jw.SessionMiddleware(page)))
```

The middleware finds or creates a Session before invoking the handler. The
application lock makes the `Get`/create/`Set` sequence atomic across simultaneous
page loads. The stored binder has its own value lock. Both tabs in one Session
share it; another Session gets its own value. Store application state or binders
in the Session, not Requests or widget trees.

`Session.Get` and `Session.Set` synchronize access to the session map. Objects
stored in it still need their own synchronization. Session storage does not
implicitly register or dirty dependency tags; use [bindings](bindings.md) and
[tags](tags.md) for values displayed by widgets.

`Request.Get` and `Request.Set` forward to its Session. Without one, `Get`
returns nil and `Set` does nothing.

## Choose another creation method

`jw.NewSession(w, r)` explicitly creates a Session and publishes its cookie to
both the response and the current HTTP request. Call it before `NewRequest`
and before writing the response. Prevent shared caching of responses carrying
the cookie. On successful replacement it clears and closes matching existing
Sessions; the processing loop must be running. Check its return value for nil.

`jw.AutoSession = true` creates an anonymous Session during a successful
WebSocket upgrade when none is attached and limits permit it. This happens
after initial page rendering, so use middleware when the initial page needs
session state. `AutoSession` does not guarantee creation when a limit is reached.

## Lifetime and limits

Sessions remain live while Requests are attached. They start with a one-minute
grace deadline; detaching a claimed Request refreshes it, while detaching an
unclaimed Request leaves it unchanged. Maintenance removes expired Sessions
without attached Requests. Cookies have no explicit expiry or MaxAge.

`Session.Reload` asks its active pages to reload. `Session.Close` invalidates
the Session and queues reloads for its associated pages, including those still
waiting to connect. It leaves stored data intact; call `Session.Clear` to remove
it. Its returned cookie can be sent in an HTTP response to remove the browser
cookie. See
[session.go](../session.go) for these contracts. `Jaws.Close` invalidates all
Sessions and clears their data.

`Request.Reload` queues a reload command for one page. The WebSocket writer sends
it and closes the connection, cancelling the Request's context. Events and
callbacks run normally until disconnection. Pending pages send the reload after
successful connection setup, including their connection callback. Session data
and cookies are preserved.

`MaxSessions` limits registered Sessions globally. `MaxSessionsPerIP` limits a
client address bucket. Both default to zero, which disables the respective
limit. A lower per-IP cap reserves capacity for other buckets. Replacement needs
a free slot under both enabled caps and preserves the old Session on refusal.

When middleware cannot create a Session, it skips the page handler. A full
per-IP bucket returns HTTP 429 while global capacity remains; the global limit
or another creation failure returns HTTP 503. Existing Sessions remain usable.
Expiry is checked every tenth maintenance pass, so expired Sessions can still
count until cleanup.

Sessions use the same IP comparison as request callbacks. Loopback addresses
compare equal. Behind a loopback proxy, configure trusted forwarding to bind to
actual client addresses. Shared addresses also share capacity limits. See
[proxy configuration](deployment.md#reverse-proxies).
