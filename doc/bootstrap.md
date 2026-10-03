# Bootstrap assets

[Documentation](README.md) · [Getting started](getting-started.md) · [Deployment](deployment.md)

`github.com/linkdata/jaws/jawsboot` embeds Bootstrap v5.3.8 CSS and the JavaScript
bundle. It serves them from the application through content-hashed URLs.

## Add Bootstrap to a page

Import `github.com/linkdata/jaws/jawsboot` and call `Setup` during JaWS
configuration, before serving HTTP requests:

```go
if err := jw.Setup(mux.Handle, "/static", jawsboot.Setup); err != nil {
	return err
}
```

Here `jw` is a `*jaws.Jaws` and `mux` is an `*http.ServeMux`. `Jaws.Setup`
registers the asset handlers and includes the returned URLs in the configured
head markup. Render that markup inside the page's `<head>`:

```html
<head>{{$.HeadHTML}}</head>
```

The [Bootstrap server example](../jawsboot/example_test.go) combines Bootstrap,
an embedded favicon, template reloading, and a bound control. It is
compile-checked but requires the illustrated template and favicon files to run.
See [Template reloading](utilities.md#reload-templates-during-development) for
development-time reloads.

## Paths and serving

The prefix is rooted and cleaned. `/static`, `static`, and `../static` all serve
under `/static`; an empty prefix serves at `/`. Returned asset URLs match the
registered paths. Characters such as braces in the prefix are literal URL path
data.

The handlers support plain and gzip responses. Source maps are not embedded;
the exact `bootstrap.bundle.min.js.map` and `bootstrap.min.css.map` paths under
the prefix return 404.

Calling `jw.Setup(nil, prefix, jawsboot.Setup)` generates the head markup
without registering handlers. Use that form only when the generated asset URLs
are served elsewhere. Calling `jawsboot.Setup` directly requires a non-nil
registration function and returns the asset URLs for the caller to use.

## Asset provenance and updates

The files are vendored from [Bootstrap](https://getbootstrap.com/) v5.3.8 and
stored gzip-compressed:

| Repository file | Upstream artifact |
| --- | --- |
| [`bootstrap.bundle.min.js.gz`](../jawsboot/assets/static/bootstrap.bundle.min.js.gz) | `bootstrap.bundle.min.js` |
| [`bootstrap.min.css.gz`](../jawsboot/assets/static/bootstrap.min.css.gz) | `bootstrap.min.css` |

To update the bundled version:

1. Replace both gzip-compressed files with the corresponding minified artifacts
   from the official Bootstrap distribution, preserving their filenames.
2. Update the version in this page, [`jawsboot/doc.go`](../jawsboot/doc.go), and
   the `assetsFS` comment in [`jawsboot.go`](../jawsboot/jawsboot.go).
3. Check the decompressed contents and any source-map names referenced by the
   artifacts. Update the exact 404 paths in `Setup` if those names change.
4. Run `go test ./jawsboot` and `go test -race ./jawsboot`. These tests check
   serving, response headers, URL/handler agreement, prefix forms, and head
   markup integration.
