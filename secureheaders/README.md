# secureheaders

`secureheaders` is an `http.Handler` middleware that writes a secure baseline of
HTTP response headers.

## Default headers

`SetHeaders` sets:

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

`BuildContentSecurityPolicy(resourceURLs)` builds a
`Content-Security-Policy` header value from known external resources.

Behavior:

- Starts with a strict baseline:
  `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';`
  `img-src 'self' data:; font-src 'self'; connect-src 'self'; `
  `frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'`.
- Adds external source expressions from `resourceURLs` by resource type:
  - `.js` -> `script-src`
  - `.css` -> `style-src`
  - image MIME types -> `img-src`
  - font MIME types -> `font-src`
  - `ws://`/`wss://` URLs -> `connect-src`

Example:

```go
u1, _ := url.Parse("https://cdn.jsdelivr.net/npm/bootstrap@5/dist/css/bootstrap.min.css")
u2, _ := url.Parse("https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.min.js")

csp, err := secureheaders.BuildContentSecurityPolicy(
	[]*url.URL{u1, u2},
)
if err != nil {
	panic(err)
}
w.Header().Set("Content-Security-Policy", csp)
```

## Extra headers

To add additional response headers, wrap your handler around the middleware
or set them in the wrapped handler itself after middleware processing.
