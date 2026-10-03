# Getting started

[Documentation](README.md) · [Next: pages and widgets](ui/README.md)

## Install

Use the Go version required by [go.mod](../go.mod) or newer. In a new directory:

```sh
go mod init example.com/hello-jaws
go get github.com/linkdata/jaws
```

Save this program as `main.go`:

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

const pageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>JaWS range</title>
  {{$.HeadHTML}}
</head>
<body>
  <label>Percent {{$.Range .Dot}}</label>
  <p>Current value: {{$.Span .Dot}}</p>
  {{$.TailHTML}}
</body>
</html>`

func main() {
    jw, err := jaws.New()
    if err != nil {
        panic(err)
    }
    defer jw.Close()
    jw.Logger = slog.Default()

    templates := template.Must(template.New("index").Parse(pageHTML))
    if err := jw.AddTemplateLookuper(templates); err != nil {
        panic(err)
    }

    var mu sync.Mutex
    percent := 50
    value := bind.New(&mu, &percent)

    go jw.Serve()
    mux := http.NewServeMux()
    mux.Handle("GET /jaws/", jw)
    mux.Handle("GET /{$}", ui.Handler(jw, "index", value))
    if err := http.ListenAndServe("localhost:8080", mux); err != nil {
        panic(err)
    }
}
```

Run it:

```sh
go mod tidy
go run .
```

Open [localhost:8080](http://localhost:8080/) in two tabs. Moving either range
updates the shared Go value and the controls in both tabs. `Range` defaults to
0–100 with step 1. This example deliberately shares one value across visitors;
use [sessions](sessions.md) for per-session state.

## How the pieces fit

`jaws.New` creates the engine. Configure it before starting `Serve`, which
processes dirty tags, broadcasts, and maintenance. Route `/jaws/` to the engine
for its assets and WebSocket callbacks.

`ui.Handler` creates a JaWS Request and renders a complete HTML document. The
template root is a `ui.With`: `.Dot` is the application's data and `$` exposes
helpers such as `Range`, `Span`, `HeadHTML`, and `TailHTML`. `HeadHTML` includes
the browser resources and connection metadata. `TailHTML` applies queued initial
attribute and class updates before the WebSocket connects; other queued
operations wait for the connection.

`bind.New` reads and writes `percent` under `mu`. The field pointer is its
dependency tag. A changed or rejected browser edit dirties that tag so every
bound control can refresh. When application code changes the field itself, release its lock
and call `jw.Dirty(&percent)`.

The same synchronized binder can serve several Requests. Each Request
constructs its own widget definitions and live Elements. Go state and dependency
tags can be shared; Requests and Elements belong to JaWS.

Continue with [pages and widgets](ui/README.md), [bindings](bindings.md), and
[tags and updates](tags.md). The [examples](examples.md) show larger applications.
Before making the server public, configure [deployment settings](deployment.md).
