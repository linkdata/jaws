---
name: jaws
description: Find the version-matched JaWS documentation for Go UI work with github.com/linkdata/jaws.
metadata:
  short-description: JaWS documentation entry point
---

# JaWS documentation

The introduction and how-to guides are in `doc/README.md` at the JaWS module
root. They cover setup, Go templates, widgets, bindings, tags, sessions, and
deployment. Exported API contracts are in Go documentation alongside the code.

In a consuming application, find its selected JaWS source, including a local
`replace`, with:

```sh
go list -m -f '{{.Dir}}' github.com/linkdata/jaws
```

Open `doc/README.md` in that directory. In a JaWS checkout, open its own
`doc/README.md`. The [online wiki](https://github.com/linkdata/jaws/tree/main/doc)
describes the development branch and may differ from the selected version.
