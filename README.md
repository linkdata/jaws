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

## UI package

The widget types are also available from `github.com/linkdata/jaws/ui`
using short names:

```go
import (
  "github.com/linkdata/jaws"
  "github.com/linkdata/jaws/ui"
)

var span *ui.Span = ui.NewSpan(jaws.MakeHTMLGetter("hello"))
```

This maps legacy names to the new package naming:

* `jaws.UiSpan` -> `ui.Span`
* `jaws.NewUiSpan(...)` -> `ui.NewSpan(...)`

### Migration regex cookbook

These regexes are intended for project-wide find/replace in editors that support
capture groups.

1. Import path move (`jaws/jaws` -> `jaws/core`)

   Find:

   ```regex
   "github\.com/linkdata/jaws/jaws"
   ```

   Replace:

   ```text
   "github.com/linkdata/jaws/core"
   ```

2. Legacy widget constructor calls (`jaws.NewUiX(...)` -> `ui.NewX(...)`)

   Find:

   ```regex
   \bjaws\.NewUi([A-Z][A-Za-z0-9_]*)\(
   ```

   Replace:

   ```text
   ui.New$1(
   ```

3. Legacy widget type names (`jaws.UiX` -> `ui.X`)

   Find:

   ```regex
   \bjaws\.Ui([A-Z][A-Za-z0-9_]*)\b
   ```

   Replace:

   ```text
   ui.$1
   ```

4. Handler helper move (`jw.Handler(name, dot)` -> `ui.NewHandler(jw, name, dot)`)
   Use targeted patterns only to avoid rewriting custom handlers.

   Find:

   ```regex
   \b([A-Za-z_][A-Za-z0-9_]*)\.Jaws\.Handler\(
   ```

   Replace:

   ```text
   ui.NewHandler($1.Jaws, 
   ```

   Find:

   ```regex
   \bjw\.Handler\(
   ```

   Replace:

   ```text
   ui.NewHandler(jw, 
   ```

5. Optional alias cleanup for migrated imports (`pkg`/`jaws` alias -> no alias)

   Find:

   ```regex
   ^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s+"github\.com/linkdata/jaws/core"\s*$
   ```

   Replace:

   ```text
   "github.com/linkdata/jaws/core"
   ```

### Tested shell commands

These commands use only standard tools (`find`, `grep`, `sed`, `perl`, `gofmt`):

```bash
# 1) import path move: jaws/jaws -> jaws/core
find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -q '"github.com/linkdata/jaws/jaws"' "$f" || continue
  sed -i.bak 's#"github.com/linkdata/jaws/jaws"#"github.com/linkdata/jaws/core"#g' "$f"
done

# 2) constructor and type rename: NewUiX/UiX -> ui.NewX/ui.X
find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -Eq '\bjaws\.NewUi[A-Z]|\bjaws\.Ui[A-Z]' "$f" || continue
  perl -i -pe 's/\bjaws\.NewUi([A-Z][A-Za-z0-9_]*)\(/ui.New$1(/g; s/\bjaws\.Ui([A-Z][A-Za-z0-9_]*)\b/ui.$1/g' "$f"
done

# 3) handler call rewrite (safe patterns only)
find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -Eq '\b[A-Za-z_][A-Za-z0-9_]*\.Jaws\.Handler\(' "$f" || continue
  perl -i -pe 's/\b([A-Za-z_][A-Za-z0-9_]*)\.Jaws\.Handler\(/ui.NewHandler($1.Jaws, /g' "$f"
done

# 4) common direct rewrite: jw.Handler(...) -> ui.NewHandler(jw, ...)
find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -Eq '\bjw\.Handler\(' "$f" || continue
  perl -i -pe 's/\bjw\.Handler\(/ui.NewHandler(jw, /g' "$f"
done

# 5) ensure ui import exists where ui.NewHandler is used
find . -name '*.go' -type f -print0 | while IFS= read -r -d '' f; do
  grep -q 'ui.NewHandler(' "$f" || continue
  grep -q '"github.com/linkdata/jaws/ui"' "$f" && continue
  perl -0777 -i -pe '
    if (/ui.NewHandler\(/ && !/"github.com\/linkdata\/jaws\/ui"/) {
      if (/import\s*\(/s) {
        s/import\s*\(\n/import (\n\t"github.com\/linkdata\/jaws\/ui"\n/s;
      } elsif (/^import\s+"[^"]+"\s*$/m) {
        s/^import\s+"([^"]+)"\s*$/import (\n\t"$1"\n\t"github.com\/linkdata\/jaws\/ui"\n)/m;
      } else {
        s/^(package\s+\w+\s*\n)/$1\nimport "github.com\/linkdata\/jaws\/ui"\n/s;
      }
    }
  ' "$f"
done

# 6) format and verify
find . -name '*.go' -type f -exec gofmt -w {} +
go test ./...

# 7) optional audit: inspect remaining custom .Handler(...) calls manually
find . -name '*.go' -type f -exec grep -nE '\.[A-Za-z_][A-Za-z0-9_]*Handler\(|\.Handler\(' {} +

# 8) remove backup files created by sed
find . -name '*.go.bak' -type f -delete
```

If `go mod` updates are requested after migration, run:

```bash
go mod tidy
```

For widget authoring guidance see `ui/README.md`.

### RequestWriter widget calls

`RequestWriter` keeps the intuitive widget helper API:

```go
rw.Span("hello", "hidden")
rw.Text(mySetter)
rw.Select(mySelectHandler, "disabled")
```

Template usage remains concise:

```gotemplate
{{$.Span "hello"}}
{{$.Text .MySetter}}
```

The explicit constructor style is also supported and is useful when you want
to prebuild or share widget instances:

```go
rw.UI(ui.NewSpan(jaws.MakeHTMLGetter("hello")), "hidden")
rw.UI(ui.NewText(mySetter))
rw.UI(ui.NewSelect(mySelectHandler), "disabled")
```

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

	http.DefaultServeMux.Handle("/", ui.NewHandler(jw, "index", jaws.Bind(&mu, &f)))
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
