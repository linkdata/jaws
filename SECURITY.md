# Security Audit Report

**Target:** https://jawsdemo.northeurope.cloudapp.azure.com/  
**Date:** 2026-04-07  
**Scope:** Web application  
**Application:** JaWS Demo v0.0.8 (jaws@v0.301.0) — Go-based real-time collaborative UI framework  
**Source:** https://github.com/linkdata/jaws (reviewed at HEAD)

---

## Executive Summary

No vulnerabilities were found. The application demonstrates a **strong security posture** with no findings above Informational severity. TLS 1.3 with post-quantum key exchange, comprehensive security headers, strict WebSocket authentication (single-use keys, IP binding, origin validation), and a server-side message whitelist combine to provide defense in depth. Source code review of the JaWS framework confirmed the empirical findings and revealed a well-designed security architecture throughout.

---

## 1. Infrastructure & Transport

### 1.1 Port Scan Results

| Port | State | Service |
|------|-------|---------|
| 80/tcp | Closed | - |
| 443/tcp | **Open** | Golang net/http, TLS 1.3 |
| 3000, 5000, 8000, 8080, 8443, 8888, 9090 | Filtered | - |

**Tool:** Nmap 7.98 (`-sV --script=http-headers,http-title,ssl-cert,ssl-enum-ciphers`)

### 1.2 TLS Configuration

| Property | Value | Rating |
|----------|-------|--------|
| Protocol | TLS 1.3 only | Excellent |
| Ciphers | AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305 | Grade A |
| Key Exchange | X25519MLKEM768 (post-quantum) | Excellent |
| Certificate | Let's Encrypt (E7), EC 256-bit, expires 2026-06-24 | Good |
| HSTS | `max-age=31536000; includeSubDomains` | Excellent |

No TLS vulnerabilities detected. Post-quantum key exchange (X25519MLKEM768) provides forward secrecy against future quantum attacks.

---

## 2. HTTP Security Headers

All headers verified via `curl -D-` and Nmap `http-headers` script.

| Header | Value | Assessment |
|--------|-------|------------|
| Content-Security-Policy | `default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'` | Strong |
| Strict-Transport-Security | `max-age=31536000; includeSubDomains` | Excellent |
| X-Frame-Options | `DENY` | Excellent |
| X-Content-Type-Options | `nosniff` | Correct |
| Referrer-Policy | `strict-origin-when-cross-origin` | Good |
| Permissions-Policy | `camera=(), microphone=(), geolocation=(), payment=()` | Good |
| X-XSS-Protection | `0` | Correct (modern best practice; relies on CSP) |
| Server | Not disclosed | Good |

### 2.1 CSP Analysis

The CSP is restrictive and well-configured:

- `script-src 'self'` blocks inline scripts, mitigating XSS even if HTML injection occurred
- `frame-ancestors 'none'` prevents clickjacking (redundant with X-Frame-Options: DENY)
- `connect-src 'self'` limits WebSocket/fetch to same origin
- `form-action 'self'` prevents form hijacking

`style-src 'unsafe-inline'` is the only relaxation. This is a necessary trade-off: the framework sets inline `style=` attributes on elements during both initial rendering and dynamic updates, and CSP nonces cannot apply to `style=` attributes (only to `<style>` elements). The theoretical risk (CSS-based data exfiltration) requires a prior HTML injection primitive, which was not found.

---

## 3. Session & Cookie Security

| Property | Value | Assessment |
|----------|-------|------------|
| Cookie name | `jawsdemo` | - |
| HttpOnly | Yes | Prevents JS access |
| Secure | Yes | HTTPS only |
| SameSite | Lax | CSRF protection |
| Path | `/` | Standard |

**Source code confirmed** (`session.go`, `newSession`): Cookie flags are set correctly in code, not relying on framework defaults.

---

## 4. Routing & HTTP Methods

### 4.1 HTTP Methods

Tested via `curl -X METHOD`:

| Method | Status | Assessment |
|--------|--------|------------|
| GET | 200 | Allowed |
| HEAD | 200 | Allowed (Go default) |
| POST | 404 | Rejected |
| PUT | 404 | Rejected |
| DELETE | 404 | Rejected |
| PATCH | 404 | Rejected |
| OPTIONS | 405 | Rejected |
| TRACE | 405 | Rejected |

Only GET and HEAD are accepted. All other methods return 404 or 405.

**Nikto confirmed:** `Allowed HTTP Methods: GET, HEAD`

### 4.2 Route Handling

Tested via `curl` and Gobuster:

| Path | Status | Content |
|------|--------|---------|
| `/` | 200 | Demo page |
| `/cars` | 200 | VIN decoder page |
| `/jaws/.jaws.{hash}.js` | 200 | Framework JavaScript |
| `/jaws/.jaws.{hash}.css` | 200 | Framework CSS |
| `/static/*` | 200 | Static assets (bootstrap, mousetrack.js, favicon) |
| `/nonexistent` | 404 | Empty |
| `/.env` | 404 | Empty |
| `/.git/HEAD` | 404 | Empty |
| `/robots.txt` | 404 | Empty |
| `/admin` | 404 | Empty |

Unknown paths correctly return 404. No information leakage on invalid routes.

### 4.3 Directory Enumeration

**Tool:** Gobuster 3.8.2 with `/usr/share/wordlists/dirb/common.txt` (4612 words)

**Result:** Only `/cars` discovered. No hidden endpoints, admin panels, or debug routes found.

---

## 5. Injection Testing

### 5.1 SQL Injection

**Tool:** SQLmap 1.10.2 (`--batch --level=3 --risk=2 --forms --crawl=2`)

Tested parameters:
- GET: `jaws.rvh`, `jaws.rvm`, `jaws.s0l`, `jaws.s0q`
- Headers: `User-Agent`, `Referer`, `Cookie`

**Result:** No SQL injection vulnerabilities. All initial boolean-based detections were confirmed as false positives caused by the real-time page content variability (WebSocket-driven dynamic updates).

### 5.2 Cross-Site Scripting (XSS)

Tested via WebSocket `Input` and `Click` messages with payloads including:
- `<script>alert(1)</script>`
- `<img src=x onerror=alert(1)>`
- `<svg onload=alert(1)>`
- `<iframe src=javascript:alert(1)>`

**Result:** No XSS vulnerabilities.

- User input sent via `Input` messages is reflected to other clients only via `Value` commands
- Client-side `jawsSetValue()` updates live form state (`value`, `checked`, or `selected`, depending on the element), not `innerHTML`
- HTML-rendering `Inner` commands only carry server-generated content, never user input
- CSP `script-src 'self'` provides defense-in-depth against inline script execution
- **Source confirmed** (`lib/ui/input_widgets.go`, `InputText.JawsUpdate`): `JawsUpdate()` calls `e.SetValue(v)`, not `e.SetInner()`

### 5.3 Cross-Site Request Forgery (CSRF)

**Tool:** Nmap `http-csrf` script

**Result:** No CSRF vulnerabilities found. WebSocket Origin validation (`request.go`, `Request.validateWebSocketOrigin`) and SameSite=Lax cookies provide protection.

---

## 6. WebSocket Security

### 6.1 Authentication & Session Binding

| Control | Implementation | Verified |
|---------|---------------|----------|
| **jawsKey** | 64-bit `crypto/rand` (2^64 keyspace), encoded as base-32 (up to 13 chars) | Code + empirical |
| **Single-claim keys** | The first successful WebSocket callback marks its Request claimed; later callbacks for that key are rejected | Empirical: second connection returns 404 |
| **IP binding** | `claim()` verifies WebSocket remote IP matches original HTTP request IP | Code review (`request.go`, `Request.claim`) |
| **Origin validation** | Scheme + host must match initial request; cross-origin returns 403 | Empirical: evil.com, null, file:// all rejected |

Tested origin validation:

| Origin Header | Result |
|---------------|--------|
| `https://jawsdemo.northeurope.cloudapp.azure.com` | 101 (accepted) |
| `https://evil.com` | 403 (rejected) |
| `https://attacker.example.com` | 403 (rejected) |
| `null` | 403 (rejected) |
| `file://` | 403 (rejected) |

### 6.2 Cookie Independence

| Test | Result |
|------|--------|
| Connect without cookie | Accepted (jawsKey is sole auth token) |
| Connect with wrong cookie | Accepted |
| Connect with cross-session cookie | Accepted |
| Connect with random/guessed key | Rejected (404) |
| Reuse consumed key | Rejected (404) |

**Assessment:** Cookie is not checked on WebSocket upgrade — authentication relies solely on the single-use jawsKey. This is a sound design: any scenario where an attacker has the jawsKey but lacks the cookie is already covered by Origin validation (blocks cross-origin) and IP binding (blocks different-IP). An XSS attacker on the same page can read the key from the meta tag, but the browser would automatically include the HttpOnly cookie in the upgrade request anyway. Cookie validation would be redundant with the existing controls.

### 6.3 Client Message Handling

**Source code** (`requestloop.go`, `Request.handleIncoming`) confirms only these message types are processed from clients:

```go
case what.Input, what.Click, what.ContextMenu, what.Set:
    rq.queueEvent(eventCallCh, ...)
case what.Remove:
    rq.handleRemove(wsmsg.Jid, wsmsg.Data)
```

All other message types (Inner, Redirect, Reload, Alert, Delete, Replace, SAttr, etc.) are silently ignored.

Tested empirically — server-only commands sent from client:

| Command | Result |
|---------|--------|
| `Inner` (set innerHTML) | Silently ignored |
| `Redirect` (navigate to URL) | Silently ignored |
| `Reload` (force page reload) | Silently ignored |
| `Alert` (show alert) | Silently ignored |
| `Delete` (remove element) | Silently ignored |
| `Replace` (replace HTML) | Silently ignored |
| `SAttr` (set attribute) | Silently ignored |

Also tested case-variant bypass attempts (`inner`, `INNER`, `redirect`, `alert`): all rejected. `what.Parse` matches command names exactly and case-sensitively, so case-variant payloads return the invalid zero value at parse time; the whitelist in the process loop is a second, authoritative gate.

### 6.4 Protocol Robustness

| Test | Result |
|------|--------|
| Malformed messages (no tabs) | Silently dropped |
| Empty messages | Silently dropped |
| Tab-only messages | Silently dropped |
| Null bytes in messages | Silently dropped |
| Unknown command types (Foo, Eval) | Silently dropped |
| Invalid UTF-8 sequences | Stripped via `strings.ToValidUTF8()` (`lib/wire/wsmsg.go`, `Parse`) |
| Oversized payload (>32 KiB) | Rejected: inbound messages are capped at 32 KiB (`webSocketReadLimit`, set via `ws.SetReadLimit` in `request.go`); a larger message fails the read and closes the connection |
| Message flood (1000 msgs in 0.03s) | Connection survived |
| 20 simultaneous connections | All accepted |
| Rapid connect/disconnect (20 cycles) | Handled gracefully |

### 6.5 JsVar State Manipulation

The `Set` message type allows clients to modify server-side JsVar state (this is the mechanism behind mouse-position sharing).

**Source code** (`lib/ui/jsvar.go`, `JsVar.JawsInput`): Client sends `Set\tJid\tpath=jsonvalue` → server unmarshals the value and applies it by path (`jq.Set()` for a non-`PathSetter` bound value) → broadcasts change.

Tested attack payloads:

| Payload | Result |
|---------|--------|
| `__proto__.polluted=true` | Rejected (invalid Go struct path) |
| `constructor.prototype.polluted=true` | Rejected |
| `../../../etc/passwd="read"` | Rejected |
| `X=999; alert(1)` | Rejected (invalid JSON) |
| `X="<script>alert(1)</script>"` | Accepted as string value; rendered in JS variable, not DOM |
| `X={"__proto__":{"polluted":true}}` | Rejected (type mismatch) |

Go's type system prevents prototype pollution — `jq.Set()` validates paths against actual struct fields and enforces type compatibility.

**Trust boundary (application responsibility):** the generic `jq.Set()` path will set *any* `json`-tagged field of the bound value and will append to a slice one element per `Set` message. The per-write size is bounded by the 32 KiB WebSocket read limit, and accumulated state is bounded by `ui.MaxClientJsVarBytes` (default 1 MiB) for non-`PathSetter` values — exceeding it aborts the `Request` with `ErrJsVarTooLarge` on the next render. There is (see I6) no per-message rate limit. A `Set` message is therefore only as constrained as the bound type. When binding a value where only some fields should be client-writable, or which contains a mutable/unbounded collection, the bound type must implement `ui.PathSetter` (`JawsSetPath`) to allow-list paths and bound lengths; the framework then routes client writes through it instead of `jq.Set()`. `jawstree.Node` is an example: it implements `JawsSetPath` to reject every path except the per-node `.selected` boolean, so a browser cannot rename nodes, mutate ids, or grow the `children` slice.

---

## 7. Information Disclosure

| Check | Result |
|-------|--------|
| Server header | Not disclosed |
| Error pages | Empty 404 responses (no stack traces, no framework info) |
| `.env` / `.git/HEAD` | 404 |
| `robots.txt` / `sitemap.xml` | 404 |
| Debug mode indicators | None (no source maps, no verbose errors) |
| JavaScript source comments | Framework name and GitHub URL visible in `.jaws.js` |
| Static asset naming | Cache-busted hashes (e.g., `.3fs1sdsh1vzi5.js`) — no version leakage |

---

## 8. Nikto Scan Results

**Tool:** Nikto 2.6.0

```
+ Target: jawsdemo.northeurope.cloudapp.azure.com:443
+ SSL: CN=jawsdemo.northeurope.cloudapp.azure.com, Issuer: Let's Encrypt E7
+ Platform: Unknown
+ Server: No banner retrieved
+ Allowed HTTP Methods: GET, HEAD
+ No CGI Directories found
+ 0 items reported
```

Clean scan — no vulnerabilities, no misconfigurations, no information disclosure.

---

## 9. Source Code Security Analysis

Source repository: https://github.com/linkdata/jaws

### 9.1 Key Generation

- `jaws.go` (`New`): Uses `crypto/rand.Reader` wrapped in `bufio.Reader`
- `requestpool.go` (`Jaws.nonZeroRandomLocked`): reads 8 bytes → `uint64`, retries on zero
- Keys are cryptographically random, non-zero, and unique within the request map

### 9.2 HTML Escaping Model

The framework uses Go's `template.HTML` type to distinguish trusted HTML from untrusted strings:

- **Input widgets** (Text, Textarea, Checkbox, Range, etc.) use `SetValue()` → sends `Value` command → client updates live form state (`value`, `checked`, or `selected`, depending on the element), not HTML
- **Display widgets** (Span, Div, Label, etc.) use `SetInner()` → sends `Inner` command → client sets `elem.innerHTML`
- `SetInner()` accepts `template.HTML`, meaning the developer has explicitly marked the content as trusted
- Initial HTML rendering escapes generated attribute values with HTML entities (`htmlio.AppendAttrValue`)

**Convenience path — plain strings are trusted too.** The HTML-inner widget
constructors and `RequestWriter` helpers (`NewSpan`/`Span`, `NewDiv`/`Div`, `A`,
`Label`, `Li`, `Td`, `Tr`, and `Button`) accept an `any` and route it
through `bind.MakeHTMLGetter`; the `Object` widget (constructed via `ui.New`)
routes its innerHTML the same way. A **plain `string`** taken by that path is treated as
**trusted HTML and is *not* escaped** — no explicit `template.HTML` cast is
required — so that markup can be passed conveniently from templates
(e.g. `{{$.Span "<i>text</i>"}}`). Values wrapped in a `bind.Getter[string]`,
`bind.Binder[string]`, or `fmt.Stringer` *are* escaped. The same trust applies to
the `named.NewBool`/`BoolArray.Add` HTML labels (typed `template.HTML`).

**Implication:** The framework itself does not create XSS vulnerabilities, but its
XSS safety is **contingent on the application developer never passing untrusted
data either as a plain `string` to an HTML-inner widget or as `template.HTML` to
`SetInner()`** — doing so would create a stored XSS condition. Wrap user input in a
`Getter`/`Stringer` (auto-escaped) or pre-escape it with
`template.HTMLEscapeString` before casting. The CSP `script-src 'self'` mitigates
this by blocking inline script execution.

### 9.3 WebSocket Message Parsing

- `lib/wire/wsmsg.go` (`Parse`): validates message structure (requires two tabs, trailing newline)
- Validates `what.What` type via `what.Parse()`
- Validates JID via `jid.ParseString()`
- JSON-unquotes data field (rejects malformed strings)
- Sanitizes with `strings.ToValidUTF8(data, "")`

### 9.4 Event Handler Safety

- `eventhandler.go` (`CallEventHandlers`): wraps handler calls in `defer recover()`, preventing panics from crashing the server
- Event handlers receive typed Go values, not raw HTML

### 9.5 Loopback IP Equivalence

- `clientip.go` (`equalIP`): treats all loopback addresses as equivalent so a reverse proxy connecting to the backend over loopback does not break session/request-key IP binding.
- Consequence: in any deployment where the backend sees only loopback peers — the common reverse-proxy topology (nginx/Caddy/load balancer → backend over `127.0.0.1`/`::1`), and shared-localhost/container/dev environments — IP binding is effectively a no-op, since every client presents the same loopback address. IP binding is defense-in-depth that supplements the single-use request key and session cookie.
- Mitigation: set `Jaws.TrustForwardedHeaders` to bind on the proxy-supplied client IP (`X-Forwarded-For` leftmost / `X-Real-IP`) instead of the loopback transport peer. Only enable this behind a single reverse proxy you control that sets these headers.
- Not exploitable in the demo's Azure VM deployment (direct client connections; no shared loopback).

---

## 10. Findings Summary

### No Issues Found

| Test | Tool/Method | Result |
|------|-------------|--------|
| SQL Injection | SQLmap (level 3, risk 2) | Not vulnerable |
| XSS (reflected/stored) | Manual WebSocket injection | Not vulnerable |
| CSRF | Nmap http-csrf, manual | Not vulnerable |
| Cross-Site WebSocket Hijacking | Manual Origin testing | Not vulnerable (403) |
| Session hijacking | Key reuse/guessing tests | Not vulnerable |
| Clickjacking | Header inspection | Protected (DENY + CSP) |
| Directory traversal | Gobuster, manual probing | No hidden paths |
| Information disclosure | Nikto, manual inspection | No leakage |
| Prototype pollution via JsVar | Manual WebSocket testing | Not vulnerable (Go type safety) |
| Command injection via WebSocket | Manual testing of all commands | Whitelist enforced |
| Protocol fuzzing | Malformed/oversized messages | Handled gracefully |

### Low Severity

None.

### Informational

| # | Finding | Details |
|---|---------|---------|
| I1 | `style-src 'unsafe-inline'` in CSP | Required by the framework's design: widgets set inline `style=` attributes in initial HTML and via `setAttribute('style', ...)` at runtime. CSP nonces cannot apply to `style=` attributes (only `<style>` elements), and eliminating all inline styles would require forbidding arbitrary style params across the framework. The theoretical risk (CSS-based data exfiltration) requires a prior HTML injection primitive, which was not found. This is a pragmatic and acceptable trade-off. |
| I2 | Mouse tracking shared across sessions | `mousetrack.js` sends cursor X/Y to server via JsVar Set; visible to co-viewers by design |
| I3 | `template.HTML` / plain-string trust boundary | Framework allows `SetInner()` with trusted HTML, and HTML-inner widgets (`Span`, `Div`, …) treat a plain `string` as trusted HTML without a cast; application developers must escape user input before casting to `template.HTML` or passing it as a plain string. See §9.2. |
| I4 | Loopback IP equivalence | `equalIP()` treats all loopback addresses as identical, so session/request IP binding is a no-op whenever the backend sees only loopback peers — including the common reverse-proxy topology, not just shared-localhost. Set `Jaws.TrustForwardedHeaders` to bind on the forwarded client IP instead. See §9.5. |
| I5 | Framework identification in JS | `/jaws/.jaws.*.js` contains `// https://github.com/linkdata/jaws` comment |
| I6 | No explicit WebSocket message rate limiting | 1000 messages in 0.04s accepted on a single connection. However, each connection requires a prior HTTP GET + TLS + WebSocket upgrade + IP validation, making connection spam expensive for the attacker. Message flood cost depends on application event handler complexity (trivial for this demo). |

---

## 11. Tools Used

| Tool | Version | Purpose |
|------|---------|---------|
| Nmap | 7.98 | Port scanning, service detection, TLS analysis, HTTP headers |
| Nikto | 2.6.0 | Web server vulnerability scanning |
| Gobuster | 3.8.2 | Directory/file enumeration |
| SQLmap | 1.10.2 | SQL injection testing |
| Python websocket-client | 1.8.0 | WebSocket protocol testing |
| Python websockets | 15.0.1 | WebSocket protocol testing |
| curl | - | HTTP method testing, header inspection |
| Source code review | - | Go source analysis of github.com/linkdata/jaws |

---

## 12. Methodology

1. **Reconnaissance:** Port scanning (Nmap), application fingerprinting, technology identification
2. **Transport security:** TLS configuration audit, HSTS verification, certificate inspection
3. **HTTP hardening:** Security header review, HTTP method testing, routing analysis
4. **Content discovery:** Directory enumeration (Gobuster), sensitive file probing
5. **Injection testing:** SQL injection (SQLmap), XSS via WebSocket, CSRF assessment
6. **WebSocket security:** Origin validation, session hijacking, protocol fuzzing, JsVar manipulation, cross-client injection, command spoofing
7. **Source code review:** Authentication flow, message parsing, HTML escaping model, event handler safety, key generation
