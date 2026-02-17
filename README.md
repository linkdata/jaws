[![build](https://github.com/linkdata/jaws/actions/workflows/build.yml/badge.svg)](https://github.com/linkdata/jaws/actions/workflows/build.yml)
[![coverage](https://github.com/linkdata/jaws/blob/coverage/main/badge.svg)](https://html-preview.github.io/?url=https://github.com/linkdata/jaws/blob/coverage/main/report.html)
[![goreport](https://goreportcard.com/badge/github.com/linkdata/jaws)](https://goreportcard.com/report/github.com/linkdata/jaws)
[![Docs](https://godoc.org/github.com/linkdata/jaws?status.svg)](https://godoc.org/github.com/linkdata/jaws)

# JaWS

Javascript and WebSockets used to create responsive webpages.

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

For widget authoring guidance see `ui/README.md`.

## Quick start

The following minimal program renders a single range input whose value
is kept on the server. Copy the snippet into a new module, run `go
mod tidy`, and start it with `go run .`. Visiting
http://localhost:8080/ demonstrates the full request lifecycle.

## Usage

```go
import (
	"html/template"
	"log/slog"
	"net/http"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/ui"
)

const indexhtml = `
<html>
<head>{{$.HeadHTML}}</head>
<body>{{$.Range .Dot}}</body>
</html>
`

func main() {
	jw, err := jaws.New() // create a default JaWS instance
	if err != nil {
		panic(err)
	}
	defer jw.Close()           // ensure we clean up
	jw.Logger = slog.Default() // optionally set the logger to use

	// parse our template and inform JaWS about it
	templates := template.Must(template.New("index").Parse(indexhtml))
	jw.AddTemplateLookuper(templates)

	go jw.Serve()                             // start the JaWS processing loop
	http.DefaultServeMux.Handle("/jaws/", jw) // ensure the JaWS routes are handled

	var mu sync.Mutex
	var f float64

	http.DefaultServeMux.Handle("/", ui.Handler(jw, "index", jaws.Bind(&mu, &f)))
	slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
}
```

Next steps when building a real application typically include:

1. Adding more templates and wiring them with `AddTemplateLookuper`.
2. Creating types that implement `JawsRender` and `JawsUpdate` so they
   can be reused as widgets.
3. Introducing sessions (see below) to keep track of user state.

### Creating HTML entities

When JawsRender() is called for a UI object, it can call
NewElement() to create new Elements while writing their initial
HTML code to the web page. Each Element is a unique instance
of a UI object bound to a specific Request, and will have a
unique HTML id.

If a HTML entity is not registered in a Request, JaWS will not
forward events from it, nor perform DOM manipulations for it.

Dynamic updates of HTML entities is done using the different methods on
the Element object when the JawsUpdate() method is called.

### Javascript events

Supported Javascript events are sent to the server and
are handled by the Element's UI type. If that didn't handle the event,
any extra objects added to the Element are invoked (in order) until one
handles the event. If none handle the event, it is ignored.

The generic event handler is `JawsEvent`. An event handler should
return `ErrEventUnhandled` if it didn't handle the event or wants
to pass it to the next handler.

* `onclick` invokes `JawsClick` if present, otherwise `JawsEvent` with `what.Click`
* `oninput` invokes `JawsEvent` with `what.Input`

## Technical notes

### WebSocket wire format notes

JaWS websocket messages are line-based and field-delimited:
`What<TAB>Jid<TAB>Data<LF>`. Keep these invariants in mind when changing
client/server protocol code:

* `what.Remove` means remove child element(s). For browser-originated `Remove`
  messages, the websocket `Jid` identifies the parent/container and `Data`
  carries the removed managed child IDs.
* `what.Replace` always uses `where + "\n" + html` in `Data`.
  `Element.Replace` encodes self-replace as empty `where`, i.e. `"\n" + html`.
* `what.Call`/`what.Set` use `path + "=" + json` inside `Data`. Embedded tabs
  or newlines in JSON break message framing; `Jaws.JsCall` compacts valid JSON
  before sending.

### HTTP request flow and associating the WebSocket

When a new HTTP request is received, create a JaWS Request using the
JaWS object's `NewRequest()` method, and then use the Request's
`HeadHTML()` method to get the HTML code needed in the `<head>` section
of the HTML page.

When the client has finished loading the document and parsed the
scripts, the JaWS JavaScript will request a WebSocket connection on
`/jaws/*`, with the `*` being the encoded `Request.JawsKey` value.

On receiving the WebSocket HTTP request, decode the key parameter from
the URL and call the JaWS object's `UseRequest()` method to retrieve the
Request created in the first step. Then call its `ServeHTTP()` method to
start up the WebSocket and begin processing JavaScript events and DOM
updates.

### Safe to call before `Serve()`

The following APIs are safe to call before starting the JaWS processing
loop (`Serve()` or `ServeWithTimeout()`):

* Construction and lifecycle: `jaws.New()`, `(*Jaws).Close()`, `(*Jaws).Done()`.
* Configuration: `(*Jaws).AddTemplateLookuper()`, `(*Jaws).RemoveTemplateLookuper()`,
  `(*Jaws).LookupTemplate()`, `(*Jaws).GenerateHeadHTML()`, `(*Jaws).Setup()`,
  `(*Jaws).FaviconURL()`.
* Inspection and logging helpers: `(*Jaws).RequestCount()`, `(*Jaws).Pending()`,
  `(*Jaws).SessionCount()`, `(*Jaws).Sessions()`, `(*Jaws).Log()`,
  `(*Jaws).MustLog()`.
* Static/ping JaWS endpoints via `(*Jaws).ServeHTTP()`:
  `/jaws/.ping`, `/jaws/.jaws.*.js`, `/jaws/.jaws.*.css`.

Broadcasting APIs are not safe before the processing loop starts. In particular,
`(*Jaws).Broadcast()` (and helpers that call it), `(*Session).Broadcast()`,
`(*Session).Reload()` and `(*Session).Close()` may block before `Serve()` or
`ServeWithTimeout()` is running.

### Routing

JaWS doesn't enforce any particular router, but it does require several
endpoints to be registered in whichever router you choose to use. All of
the endpoints start with "/jaws/", and `Jaws.ServeHTTP()` will handle all
of them.

* `/jaws/\.jaws\.[0-9a-z]+\.css`

  Serves the built-in JaWS stylesheet.

  The response should be cached indefinitely.

* `/jaws/\.jaws\.[0-9a-z]+\.js`

  Serves the built-in JaWS client-side JavaScript.

  The response should be cached indefinitely.

* `/jaws/[0-9a-z]+` (and `/jaws/[0-9a-z]+/noscript`)

  The WebSocket endpoint. The trailing string must be decoded using
  `jaws.JawsKeyValue()` and then the matching JaWS Request retrieved
  using the JaWS object's `UseRequest()` method.

  If the Request is not found, return a **404 Not Found**, otherwise 
  call the Request `ServeHTTP()` method to start the WebSocket and begin
  processing events and updates.

* `/jaws/.ping`

  This endpoint is called by the Javascript while waiting for the server to
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
http.DefaultServeMux.Handle("/jaws/", jw)
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

HTML output elements (e.g. `jaws.RequestWriter.Div()`) require a `jaws.HTMLGetter` or something that can
be made into one using `jaws.MakeHTMLGetter()`.

In order of precedence, this can be:
* `jaws.HTMLGetter`: `JawsGetHTML(*Element) template.HTML` to be used as-is.
* `jaws.Getter[string]`: `JawsGet(*Element) string` that will be escaped using `html.EscapeString`.
* `jaws.Formatter`: `Format("%v") string` that will be escaped using `html.EscapeString`.
* `fmt.Stringer`: `String() string` that will be escaped using `html.EscapeString`.
* a static `template.HTML` or `string` to be used as-is with no HTML escaping.
* everything else is rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.

You can use `jaws.Bind().FormatHTML()`, `jaws.HTMLGetterFunc()` or `jaws.StringGetterFunc()` to build a custom renderer
for trivial rendering tasks, or define a custom type implementing `HTMLGetter`.

### Data binding

HTML input elements (e.g. `jaws.RequestWriter.Range()`) require bi-directional data flow between the server and the browser.
The first argument to these is usually a `Setter[T]` where `T` is one of `string`, `float64`, `bool` or `time.Time`. It can
also be a `Getter[T]`, in which case the HTML element should be made read-only.

Since all data access need to be protected with locks, you will usually use `jaws.Bind()` to create a `jaws.Binder[T]`
that combines a (RW)Locker and a pointer to a value of type `T`. It also allows you to add chained setters,
getters and on-success handlers.

### Session handling

JaWS has non-persistent session handling integrated. Sessions won't 
be persisted across restarts and must have an expiry time.

Use one of these patterns:

* Wrap page handlers with `Jaws.Session(handler)` to ensure a session exists.
* Call `Jaws.NewSession(w, r)` explicitly to create and attach a fresh session cookie.

When subsequent Requests are created with `NewRequest()`, if the
HTTP request has the cookie set and comes from the correct IP,
the new Request will have access to that Session.

Session key-value pairs can be accessed using `Request.Set()` and
`Request.Get()`, or directly using a `Session` object. It's safe to
do this if there is no session; `Get()` will return nil, and `Set()`
will be a no-op.

Sessions are bound to the client IP. Attempting to access an existing 
session from a new IP will fail.

No data is stored in the client browser except the randomly generated 
session cookie. You can set the cookie name in `Jaws.CookieName`, the
default is `jaws`.

### A note on the Context

The Request object embeds a context.Context inside its struct,
contrary to recommended Go practice.

The reason is that there is no unbroken call chain from the time the Request
object is created when the initial HTTP request comes in and when it is
requested during the Javascript WebSocket HTTP request.

### Security of the WebSocket callback

Each JaWS request gets a unique 64-bit random value assigned to it when you 
create the Request object. This value is written to the HTML output so it
can be read by the Javascript, and used to construct the WebSocket callback
URL.

Once the WebSocket call comes in, the value is consumed by that request,
and is no longer valid until, theoretically, another Request gets the same
random value. And that's fine, since JaWS guarantees that no two Requests
waiting for WebSocket calls can have the same value at the same time.

In addition to this, Requests that are not claimed by a WebSocket call get
cleaned up at regular intervals. By default an unclaimed Request is 
removed after 10 seconds.

In order to guess (and thus hijack) a WebSocket you'd have to make on the
order of 2^63 requests before the genuine request comes in, or 10 seconds
pass assuming you can reliably prevent the genuine WebSocket request.

### Dependencies

We try to minimize dependencies outside of the standard library.

* Depends on https://github.com/coder/websocket for WebSocket functionality.
* Depends on https://github.com/linkdata/deadlock if race detection is enabled.

## Learn more

* Browse the [Go package documentation](https://pkg.go.dev/github.com/linkdata/jaws)
  for an API-by-API overview.
* Inspect the [`example_test.go`](./example_test.go) file for executable
  examples that can be run with `go test`.
* Explore the [demo application](https://github.com/linkdata/jawsdemo)
  to see a more complete, heavily commented project structure.
