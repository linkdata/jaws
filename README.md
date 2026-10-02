[![build](https://github.com/linkdata/jaws/actions/workflows/build.yml/badge.svg)](https://github.com/linkdata/jaws/actions/workflows/build.yml)
[![coverage](https://github.com/linkdata/jaws/blob/gitcoverage/main/badge.svg)](https://html-preview.github.io/?url=https://github.com/linkdata/jaws/blob/gitcoverage/main/report.html)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/linkdata/jaws/badge)](https://scorecard.dev/viewer/?uri=github.com/linkdata/jaws)
[![Docs](https://godoc.org/github.com/linkdata/jaws?status.svg)](https://godoc.org/github.com/linkdata/jaws)

# JaWS

JavaScript and WebSockets for responsive web pages built in Go.

JaWS renders HTML from server-side application state, binds controls to Go
values, and sends targeted DOM updates over WebSockets. It integrates with
`net/http`, Go templates, and routers that accept `http.Handler`.

```sh
go get github.com/linkdata/jaws
```

## Documentation

The [documentation wiki](doc/README.md) is the introduction and how-to guide for
both people and coding assistants.

- [Getting started](doc/getting-started.md): run a complete application.
- [Pages and widgets](doc/ui/README.md), [bindings](doc/bindings.md), and [tags](doc/tags.md): build interactive pages.
- [Sessions](doc/sessions.md) and [deployment](doc/deployment.md): configure an application for users.
- [Examples](doc/examples.md): small examples and collaborative Minesweeper.
- [Go API reference](https://pkg.go.dev/github.com/linkdata/jaws): exported types and methods.

The [demo application](https://github.com/linkdata/jawsdemo) shows a complete
project. Repository checks are described in [development](doc/development.md).

## Coding-assistant entry point

The optional [JaWS skill](.agents/skills/jaws/SKILL.md) links to the same wiki.
To install it, copy `.agents/skills/jaws/` from the checkout into
`~/.agents/skills/jaws/`. Use documentation from the module version selected by
the application.
