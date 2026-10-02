# JaWS documentation

JaWS builds interactive web pages from Go state. Go templates render the page,
widgets bind controls to server values, and a WebSocket carries browser events
and targeted DOM updates. Application state lives on the server; the browser
holds the displayed values and transient interaction state.

## Design checklist

- **State:** pass application objects directly to templates; use their methods for [bindings, actions, and computed content](bindings.md).
- **Structure:** use [standard templates](ui/README.md#render-a-page-or-partial) for fixed structure; use [containers](ui/controls.md#lists-and-selections) when children change.
- **Attributes:** keep static attributes as template strings; use member functions for [dynamic initial attributes](ui/README.md#choose-initial-attributes). Choose how [later attribute changes](ui/custom.md#update-attributes-after-rendering) are applied.
- **Dependencies:** register every state dependency an Element uses. Have mutations identify the affected [tags](tags.md); dirty them after unlocking.
- **Lifetime:** construct [fresh widgets per Request](ui/README.md#widget-lifetime-and-identity), sharing synchronized application state and stable tags.
- **HTML:** [escape user-provided values](bindings.md#html-and-attribute-safety); raw strings passed to JaWS HTML helpers are trusted markup.
- **Verification:** check initial markup and [complete wire output](testing.md), including no-ops, at the intended application size.

## Start here

1. [Getting started](getting-started.md): install JaWS and run a complete application.
2. [Pages and widgets](ui/README.md): render templates, compose controls, and manage dynamic children.
3. [Bindings](bindings.md): read, edit, validate, and format application values.
4. [Tags and updates](tags.md): refresh the controls affected by a state change.

These guides describe this source checkout. Symbol-level contracts are in the
[Go API reference](https://pkg.go.dev/github.com/linkdata/jaws); local `go doc`
reads the version selected by your module.

## Build an application

- [Runtime and requests](runtime.md): configuration, routes, events, contexts, and status counts.
- [Sessions](sessions.md): session data, cookies, lifetime, and limits.
- [Deployment](deployment.md): logging, authorization, escaping, proxies, and resource limits.
- [Browser integration](browser.md): client resources, browser behavior, and JavaScript hooks.
- [Bootstrap](bootstrap.md): add the bundled styles and scripts.
- [Utilities](utilities.md): HTML output, named selections, and template reloading.
- [Examples](examples.md): small examples and the collaborative Minesweeper application.

## Test and extend

- [Testing](testing.md): isolated Requests, Elements, and wire-message assertions.
- [Development](development.md): run repository tests, generators, and checks.
- [Transport](transport.md): commands, record framing, Request keys, and Element IDs.

## Package map

| Package | Guide |
| --- | --- |
| [`jaws`](../doc.go) | [Runtime](runtime.md), [sessions](sessions.md), [deployment](deployment.md) |
| [`lib/ui`](../lib/ui) | [Pages and widgets](ui/README.md) |
| [`lib/bind`](../lib/bind) | [Bindings](bindings.md) |
| [`lib/tag`](../lib/tag) | [Tags and updates](tags.md) |
| [`lib/assets`](../lib/assets) | [Browser integration](browser.md) |
| [`lib/htmlio`](../lib/htmlio), [`lib/named`](../lib/named), [`lib/templatereloader`](../lib/templatereloader) | [Utilities](utilities.md) |
| [`lib/jid`](../lib/jid), [`lib/key`](../lib/key), [`lib/what`](../lib/what), [`lib/wire`](../lib/wire) | [Transport](transport.md) |
| [`jawsboot`](../jawsboot) | [Bootstrap](bootstrap.md) |
| [`jawstest`](../jawstest) | [Testing](testing.md) |
| [`examples`](../examples), [`examples/minesweeper`](../examples/minesweeper) | [Examples](examples.md) |

[Project overview](../README.md)
