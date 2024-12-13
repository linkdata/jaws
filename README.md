[![build](https://github.com/linkdata/jaws/actions/workflows/go.yml/badge.svg)](https://github.com/linkdata/jaws/actions/workflows/go.yml)
[![coverage](https://coveralls.io/repos/github/linkdata/jaws/badge.svg?branch=main)](https://coveralls.io/github/linkdata/jaws?branch=main)
[![goreport](https://goreportcard.com/badge/github.com/linkdata/jaws)](https://goreportcard.com/report/github.com/linkdata/jaws)
[![Docs](https://godoc.org/github.com/linkdata/jaws?status.svg)](https://godoc.org/github.com/linkdata/jaws)

# JaWS

Javascript and WebSockets used to create responsive webpages.

* Moves web application state fully to the server.
* Does not trust the web browser or the Javascript.
* Binds application data to UI elements using user-definable 'tags'.

There is a [demo application](https://github.com/linkdata/jawsdemo)
with plenty of comments to use as a tutorial.

## Usage

```go
import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/linkdata/jaws"
)

const indexhtml = `
<html>
<head>{{$.HeadHTML}}</head>
<body>{{$.Range .Dot}}</body>
</html>
`

func main() {
	jw := jaws.New()           // create a default JaWS instance
	defer jw.Close()           // ensure we clean up
	jw.Logger = slog.Default() // optionally set the logger to use

	// parse our template and inform JaWS about it
	templates := template.Must(template.New("index").Parse(indexhtml))
	jw.AddTemplateLookuper(templates)

	go jw.Serve()                             // start the JaWS processing loop
	http.DefaultServeMux.Handle("/jaws/", jw) // ensure the JaWS routes are handled

	var f jaws.Float // somewhere to store the slider data
	http.DefaultServeMux.Handle("/", jw.Handler("index", &f))
	slog.Error(http.ListenAndServe("localhost:8080", nil).Error())
}
```

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

### HTTP request flow and associating the WebSocket

When a new HTTP request is received, create a JaWS Request using the JaWS
object's `NewRequest()` method, and then use the Request's `HeadHTML()` 
method to get the HTML code needed in the HEAD section of the HTML page.

When the client has finished loading the document and parsed the scripts,
the JaWS Javascript will request a WebSocket connection on `/jaws/*`, 
with the `*` being the encoded Request.JawsKey value.

On receiving the WebSocket HTTP request, decode the key parameter from 
the URL and call the JaWS object's `UseRequest()` method to retrieve the
Request created in the first step. Then call it's `ServeHTTP()` method to
start up the WebSocket and begin processing Javascript events and DOM updates.

### Routing

JaWS doesn't enforce any particular router, but it does require several
endpoints to be registered in whichever router you choose to use. All of
the endpoints start with "/jaws/", and `Jaws.ServeHTTP()` will handle all
of them.

* `/jaws/jaws.*.js`

  The exact URL is the value of `jaws.JavascriptPath`. It must return
  the client-side Javascript, the uncompressed contents of which can be had with
  `jaws.JavascriptText`, or a gzipped version with `jaws.JavascriptGZip`.

  The response should be cached indefinitely.

* `/jaws/[0-9a-z]+`

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

  If you don't have a JaWS object, or if it's completion channel is closed (see
  `Jaws.Done()`), return **503 Service Unavailable**. If you're ready to serve
  requests, return **204 No Content**.
  
  The response should not be cached.

Handling the routes with the standard library's `http.DefaultServeMux`:

```go
jw := jaws.New()
defer jw.Close()
go jw.Serve()
http.DefaultServeMux.Handle("/jaws/", jw)
```

Handling the routes with [Echo](https://echo.labstack.com/):

```go
jw := jaws.New()
defer jw.Close()
go jw.Serve()
router := echo.New()
router.GET("/jaws/*", func(c echo.Context) error {
  jw.ServeHTTP(c.Response().Writer, c.Request())
  return nil
})
```

### HTML rendering

HTML output elements (e.g. `jaws.RequestWriter.Div()`) require a `jaws.HtmlGetter` or something that can
be made into one using `jaws.MakeHtmlGetter()`.

In order of precedence, this can be:
* `jaws.HtmlGetter`: `JawsGetHtml(*Element) template.HTML` to be used as-is.
* `jaws.Getter[template.HTML]`: `JawsGet(*Element) template.HTML` to be used as-is.
* `jaws.StringGetter`: `JawsGetString(*Element) string` that will be escaped using `html.EscapeString`.
* `jaws.Getter[string]`: `JawsGet(*Element) string` that will be escaped using `html.EscapeString`.
* `jaws.AnyGetter`: `JawsGetAny(*Element) any` that will be rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.
* `fmt.Stringer`: `String() string` that will be escaped using `html.EscapeString`.
* a static `template.HTML` or `string` to be used as-is with no HTML escaping.
* everything else is rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.

### Data binding

HTML input elements (e.g. `jaws.RequestWriter.Range()`) require bi-directional data flow between the server and the browser.
The first argument to these is usually a `Setter[T]` where `T` is one of `string`, `float64`, `bool` or `time.Time`. It can
also be a `Getter[T]`, in which case the HTML element should be made read-only.

You can also use a `Binder[T]` that combines a (RW)Locker and a pointer to the value, and allows you to add chained setters,
getters and on-success handlers. It can be used as a `jaws.HtmlGetter`.

### Session handling

JaWS has non-persistent session handling integrated. Sessions won't 
be persisted across restarts and must have an expiry time. A new
session is created with `EnsureSession()` and sending it's `Cookie()`
to the client browser.

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

The Request object embeds a context.Context inside it's struct,
contrary to recommended Go practice.

The reason is that there is no unbroken call chain from the time the Request
object is created when the initial HTTP request comes in and when it's 
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
