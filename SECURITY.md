# Security Audit Report

**Target:** https://jawsdemo.northeurope.cloudapp.azure.com/  
**Date:** 2026-04-07  
**Scope:** Web application only
**Application:** JaWS Demo v0.0.8 (jaws@v0.301.0) — Go-based real-time collaborative UI framework  
**Source:** https://github.com/linkdata/jaws (reviewed at HEAD)

---

## Executive Summary

The application demonstrates a **strong security posture**. No critical or high-severity vulnerabilities were found. The server-side hardening (security headers, TLS 1.3, cookie flags, WebSocket authentication) is well above average. Source code review confirmed the empirical findings and revealed a well-designed security architecture with defense in depth.

**Overall Risk Rating: Low**

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

**Finding (Low):** `style-src 'unsafe-inline'` is the only relaxation. CSS injection could theoretically leak data via attribute selectors, though exploitation is difficult with this application's structure. Consider using style nonces if feasible.

---

## 3. Session & Cookie Security

| Property | Value | Assessment |
|----------|-------|------------|
| Cookie name | `jawsdemo` | - |
| HttpOnly | Yes | Prevents JS access |
| Secure | Yes | HTTPS only |
| SameSite | Lax | CSRF protection |
| Path | `/` | Standard |

**Source code confirmed** (`session.go:31-38`): Cookie flags are set correctly in code, not relying on framework defaults.

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
- Client-side `jawsSetValue()` uses `elem.value` / `elem.textContent` (not `innerHTML`)
- HTML-rendering `Inner` commands only carry server-generated content, never user input
- CSP `script-src 'self'` provides defense-in-depth against inline script execution
- **Source confirmed** (`input_widgets.go:48-52`): `JawsUpdate()` calls `e.SetValue(v)`, not `e.SetInner()`

### 5.3 Cross-Site Request Forgery (CSRF)

**Tool:** Nmap `http-csrf` script

**Result:** No CSRF vulnerabilities found. WebSocket Origin validation (`request.go:879-918`) and SameSite=Lax cookies provide protection.

---

## 6. WebSocket Security

### 6.1 Authentication & Session Binding

| Control | Implementation | Verified |
|---------|---------------|----------|
| **jawsKey** | 64-bit `crypto/rand` (2^64 keyspace), encoded as 13-char base36 | Code + empirical |
| **Single-use keys** | Key removed from map on first WebSocket connection | Empirical: second connection returns 404 |
| **IP binding** | `claim()` verifies WebSocket remote IP matches original HTTP request IP | Code review (`request.go:88-108`) |
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

**Assessment:** Cookie is not checked on WebSocket upgrade — authentication relies solely on the single-use jawsKey. This is acceptable given the key's high entropy and single-use property, but binding cookie to key would add defense-in-depth.

### 6.3 Client Message Handling

**Source code** (`request.go:594-601`) confirms only four message types are processed from clients:

```go
case what.Input, what.Click, what.Set:
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

Also tested case-insensitive bypass attempts (`inner`, `INNER`, `redirect`, `alert`): all silently ignored despite `what.Parse` accepting them case-insensitively. The whitelist in the process loop is the authoritative gate.

### 6.4 Protocol Robustness

| Test | Result |
|------|--------|
| Malformed messages (no tabs) | Silently dropped |
| Empty messages | Silently dropped |
| Tab-only messages | Silently dropped |
| Null bytes in messages | Silently dropped |
| Unknown command types (Foo, Eval) | Silently dropped |
| Invalid UTF-8 sequences | Stripped via `strings.ToValidUTF8()` (`wsmsg.go:63`) |
| Oversized payload (1MB) | Accepted, no crash |
| Message flood (1000 msgs in 0.03s) | Connection survived |
| 20 simultaneous connections | All accepted |
| Rapid connect/disconnect (20 cycles) | Handled gracefully |

### 6.5 JsVar State Manipulation

The `Set` message type allows clients to modify server-side JsVar state (this is the mechanism behind mouse-position sharing).

**Source code** (`jsvar.go:196-207`): Client sends `Set\tJid\tpath=jsonvalue` → server calls `jq.Set()` on Go struct → broadcasts change.

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

- `jaws.go:116-118`: Uses `crypto/rand.Reader` wrapped in `bufio.Reader`
- `jaws.go:270-278`: `nonZeroRandomLocked()` reads 8 bytes → `uint64`, retries on zero
- Keys are cryptographically random, non-zero, and unique within the request map

### 9.2 HTML Escaping Model

The framework uses Go's `template.HTML` type to distinguish trusted HTML from untrusted strings:

- **Input widgets** (Text, Checkbox, Range, etc.) use `SetValue()` → sends `Value` command → client sets `elem.value` (safe, no HTML parsing)
- **Display widgets** (Span, Div, Label, etc.) use `SetInner()` → sends `Inner` command → client sets `elem.innerHTML`
- `SetInner()` accepts `template.HTML`, meaning the developer has explicitly marked the content as trusted
- Initial HTML rendering escapes attribute values via `strconv.AppendQuote()` (`writehtml.go:48-53`)

**Implication:** The framework itself does not create XSS vulnerabilities. However, an application developer who passes unescaped user input as `template.HTML` to `SetInner()` would create a stored XSS condition. The CSP `script-src 'self'` mitigates this by blocking inline script execution.

### 9.3 WebSocket Message Parsing

- `wsmsg.go:46-73`: `Parse()` validates message structure (requires two tabs, trailing newline)
- Validates `what.What` type via `what.Parse()`
- Validates JID via `jid.ParseString()`
- JSON-unquotes data field (rejects malformed strings)
- Sanitizes with `strings.ToValidUTF8(data, "")`

### 9.4 Event Handler Safety

- `eventhandler.go:86-95`: `CallEventHandlers()` wraps handler calls in `defer recover()`, preventing panics from crashing the server
- Event handlers receive typed Go values, not raw HTML

### 9.5 Loopback IP Equivalence

- `jaws.go:736-738`: `equalIP()` treats `127.0.0.1` and `::1` as equivalent
- Relevant only in shared-localhost deployments (containers, dev environments)
- Not exploitable in the demo's Azure VM deployment

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

| # | Finding | Details | Recommendation |
|---|---------|---------|----------------|
| L1 | `style-src 'unsafe-inline'` in CSP | Allows inline styles; could theoretically enable CSS-based data exfiltration | Use style nonces or hashes if feasible |
| L2 | No WebSocket rate limiting | 1000 messages in 0.03s accepted; 20 simultaneous connections allowed | Consider per-connection message rate limits and per-IP connection caps |

### Informational

| # | Finding | Details |
|---|---------|---------|
| I1 | Cookie not validated on WebSocket upgrade | Auth relies solely on jawsKey; cookie is ignored. Acceptable given key entropy (2^64) and single-use property |
| I2 | Mouse tracking shared across sessions | `mousetrack.js` sends cursor X/Y to server via JsVar Set; visible to co-viewers by design |
| I3 | `template.HTML` trust boundary | Framework allows `SetInner()` with trusted HTML; application developers must escape user input before casting to `template.HTML` |
| I4 | Loopback IP equivalence | `equalIP()` treats all loopback addresses as identical; only relevant in shared-localhost deployments |
| I5 | Framework identification in JS | `/jaws/.jaws.*.js` contains `// https://github.com/linkdata/jaws` comment |

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
