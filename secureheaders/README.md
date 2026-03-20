# secureheaders

`secureheaders` is an `http.Handler` middleware that writes a secure baseline of
HTTP response headers.

## Default headers

`DefaultSetHeaders` sets:

- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy: default-src 'self'; frame-ancestors 'none'`
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-Xss-Protection: 0`
- `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=()`

If the request is considered HTTPS, it also sets:

- `Strict-Transport-Security: max-age=31536000; includeSubDomains`

## Usage

```go
mux := http.NewServeMux()
mux.Handle("GET /", secureheaders.Middleware{
	Handler:               myHandler,
	TrustForwardedHeaders: true,
})
```

`TrustForwardedHeaders` controls whether forwarded headers are trusted when
checking if a request is secure.

Set it to `true` only when forwarding headers are set and sanitized by trusted
infrastructure (for example, your reverse proxy).

## Security detection

`RequestIsSecure(r, trustForwardedHeaders)` always trusts `r.TLS != nil`.

When `trustForwardedHeaders` is `true`, it also checks:

- `X-Forwarded-Ssl: on`
- `Front-End-Https: on`
- `X-Forwarded-Proto`
- `Forwarded` (`proto=https`)

For list-valued forwarding headers, the first hop is used.

## CSP builder

`BuildContentSecurityPolicy(resourceURLs, listenURL)` builds a
`Content-Security-Policy` header value from known external resources and the
listener URL used for websocket connections.

Behavior:

- Starts with a strict baseline:
  `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';`
  `img-src 'self' data:; font-src 'self'; connect-src 'self';`
  `frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'`.
- Adds external source expressions from `resourceURLs` by resource type:
  - `.js` -> `script-src`
  - `.css` -> `style-src`
  - image MIME types -> `img-src`
  - font MIME types -> `font-src`
  - `ws://`/`wss://` URLs -> `connect-src`
- Adds a websocket source from `listenURL` host:
  - `https://host[:port]` -> `wss://host[:port]`
  - `http://host[:port]` -> `ws://host[:port]`
- Returns an error if `listenURL` cannot be parsed.

Example:

```go
u1, _ := url.Parse("https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css")
u2, _ := url.Parse("https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js")

csp, err := secureheaders.BuildContentSecurityPolicy([]*url.URL{u1, u2}, "https://listen.example.com:8443/ws")
if err != nil {
	panic(err)
}
w.Header().Set("Content-Security-Policy", csp)
```

## Custom header writer

The middleware calls `SetHeaders` before invoking the wrapped handler.

By default, `SetHeaders` points to `DefaultSetHeaders`, but you can override it
if you need custom behavior:

```go
secureheaders.SetHeaders = func(w http.ResponseWriter, isHTTPS bool) {
	secureheaders.DefaultSetHeaders(w, isHTTPS)
	w.Header()["Cross-Origin-Opener-Policy"] = []string{"same-origin"}
}
```
