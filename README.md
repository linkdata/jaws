[![build](https://github.com/linkdata/jaws/actions/workflows/build.yml/badge.svg)](https://github.com/linkdata/jaws/actions/workflows/build.yml)
[![coverage](https://github.com/linkdata/jaws/blob/gitcoverage/main/badge.svg)](https://html-preview.github.io/?url=https://github.com/linkdata/jaws/blob/gitcoverage/main/report.html)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/linkdata/jaws/badge)](https://scorecard.dev/viewer/?uri=github.com/linkdata/jaws)
[![Docs](https://godoc.org/github.com/linkdata/jaws?status.svg)](https://godoc.org/github.com/linkdata/jaws)

# JaWS

JavaScript and WebSockets for creating responsive webpages.

JaWS embraces a "server holds the truth" philosophy and keeps the complexity
of modern browser applications on the backend. The client-side script becomes
a thin transport layer that faithfully relays events and DOM updates.

## Features

* Moves web application state fully to the server.
* Keeps the browser intentionally dumb -- no implicit trust in JavaScript logic
  running on the client.
* Binds application data to UI elements using user-defined tags and type-aware
  binders.
* Integrates with the standard library as well as third-party routers such as
  Echo.
* Ships with a small standard library of extensible UI widgets and helpers.

The [demo application](https://github.com/linkdata/jawsdemo) is a commented,
complete example.

## Installation

JaWS is distributed as a standard Go module:

```bash
go get github.com/linkdata/jaws
```

For the standard widget APIs, see the
[`lib/ui` package documentation](https://pkg.go.dev/github.com/linkdata/jaws/lib/ui).

### AI skill

This repository includes an AI skill under `.agents/skills/jaws/`.
To install it in your local AI skills tree, copy both `SKILL.md` and
`agents/openai.yaml` into `~/.agents/skills/jaws/`.

Copying from a JaWS checkout keeps the skill baseline matched to that source.
The commands below install the current development skill from `main`; when
versioned source is available, its adjacent `AI.md` guides are canonical for
version-specific behavior.

Using `curl`:

```bash
mkdir -p "$HOME/.agents/skills/jaws/agents"
curl -fsSL https://raw.githubusercontent.com/linkdata/jaws/main/.agents/skills/jaws/SKILL.md \
	-o "$HOME/.agents/skills/jaws/SKILL.md"
curl -fsSL https://raw.githubusercontent.com/linkdata/jaws/main/.agents/skills/jaws/agents/openai.yaml \
	-o "$HOME/.agents/skills/jaws/agents/openai.yaml"
```

## Quick start

The following minimal program renders a single range input whose value stays
on the server. Copy the snippet into a new module, run `go mod tidy`, and start
it with `go run .`. Visiting <http://localhost:8080/> demonstrates the full
request lifecycle.

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

Next steps usually include adding templates with `AddTemplateLookuper`, creating
types that implement `JawsRender` and `JawsUpdate`, and introducing sessions for
per-user state.

## Production guidance

Before deploying a JaWS application, review the [production hardening
guidance](./AI.md#production-hardening).

## AI and maintainer guidance

The version-matched [AI guidance](./AI.md) documents implementation invariants,
lifecycle details, and links to the guide for every package. Exported API
contracts remain in the [Go package documentation](https://pkg.go.dev/github.com/linkdata/jaws).

## Dependencies

JaWS keeps dependencies outside the standard library to a minimum:

* [coder/websocket](https://github.com/coder/websocket) provides WebSocket
  functionality.
* [linkdata/staticserve](https://github.com/linkdata/staticserve) serves hashed
  static assets.
* [linkdata/jq](https://github.com/linkdata/jq) provides JSON path access for
  `JsVar` values.
* [linkdata/secureheaders](https://github.com/linkdata/secureheaders) provides
  the security-header baseline.
* [linkdata/deadlock](https://github.com/linkdata/deadlock) provides debug-aware
  locks.

## Learn more

* Browse the [Go package documentation](https://pkg.go.dev/github.com/linkdata/jaws)
  for an API-by-API overview.
* Read the [AI and maintainer guidance](./AI.md) for implementation details and
  the complete package-guide index.
* Inspect the compile-checked [examples](./examples/example_test.go) to copy and
  adapt the setup sequence.
* Explore the [demo application](https://github.com/linkdata/jawsdemo) for a more
  complete project structure.
