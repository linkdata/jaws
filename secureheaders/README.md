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
