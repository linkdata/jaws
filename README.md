[![build](https://github.com/linkdata/jaws/actions/workflows/go.yml/badge.svg)](https://github.com/linkdata/jaws/actions/workflows/go.yml)
[![coverage](https://coveralls.io/repos/github/linkdata/jaws/badge.svg?branch=main)](https://coveralls.io/github/linkdata/jaws?branch=main)
[![goreport](https://goreportcard.com/badge/github.com/linkdata/jaws)](https://goreportcard.com/report/github.com/linkdata/jaws)
[![Docs](https://godoc.org/github.com/linkdata/jaws?status.svg)](https://godoc.org/github.com/linkdata/jaws)

# JaWS

Javascript and WebSockets used to create responsive webpages.

## HTTP request flow and associating the WebSocket

When a new HTTP request is received, create a JaWS Request using the JaWS
object's `NewRequest()` method, and then use the Request's `HeadHTML()` 
method to get the HTML code needed in the HEAD section of the HTML page.

When the client has finished loading the document and parsed the scripts,
the JaWS Javascript will request a WebSocket connection on `/jaws/*`, 
with the `*` being the encoded Request.JawsKey value.

On receiving the WebSocket HTTP request, decode the key parameter from 
the URL and call the JaWS object's `UseRequest()` method to retrieve the
Request created in the first step. Then call it's `ServeHTTP()` method to
start up the WebSocket and begin processing Javascript events and DOM updates.

## Routing

JaWS doesn't enforce any particular router, but it does require several
endpoints to be registered in whichever router you choose to use. We do
provide a helper function for [Echo](https://echo.labstack.com/) with
`jawsecho.Setup()`.

* `/jaws/jaws.*.js`

  The exact URL is the value of `jaws.JavascriptPath`. It must return
  the client-side Javascript, the uncompressed contents of which can be had with
  `jaws.JavascriptText`, or a gzipped version with `jaws.JavascriptGZip`.

  The response should be cached indefinitely.

* `/jaws/[0-9a-z]+`

  The WebSocket endpoint. The trailing string must be decoded using 
  `jaws.JawsKeyValue()` and then the matching JaWS Request retrieved
  using the JaWS object's `UseRequest()` method.

  If the Request is not found, return a **404 Not Found**, otherwise 
  call the Request `ServeHTTP()` method to start the WebSocket and begin 
  processing events and updates.

* `/jaws/.ping`

  This endpoint is called by the Javascript while waiting for the server to
  come online. This is done in order to not spam the WebSocket endpoint with
  connection requests, and browsers are better at handling XHR requests failing.

  If you don't have a JaWS object, or if it's completion channel is closed (see
  `Jaws.Done()`), return **503 Service Unavailable**. If you're ready to serve
  requests, return **200 OK**.
  
  The response should not be cached.

## Registering HTML entities and Javascript events

The application registers the HTML entities it wants to interact with
*per request*, usually while rendering the HTML template. If a HTML entity
is not registered in a Request, JaWS will not forward events from it,
nor perform DOM manipulations for it.

Dynamic updates of HTML entities is done using the different methods on
the JaWS object and Request object. If the JaWS object is used to update
HTML entities, all Requests will receive the update request. If the Request 
object's methods are used, the update is forwarded to to all *other* Requests.

Each HTML entity registered with JaWS will have the `jid` attribute set in
the generated HTML code with the same value as it's JaWS id when it was
registered. Unlike HTML ID's you can have multiple HTML entities with
the same `jid`, and all will be affected by DOM updates.

## Session handling

JaWS has non-persistent session handling integrated. Sessions won't 
be persisted across restarts and must have an expiry time.

Sessions are bound to the client IP. Attempting to access an existing 
session from a new IP will fail as if the session does not exist.

No data is stored in the client browser except the randomly generated 
session cookie. You can set the cookie name in `Jaws.CookieName`, the
default is `jaws`.

Session key-value pairs can be accessed using `Request.Set()` and
`Request.Get()`, or directly using a `Session` object.

## A note on the Context

The Request object embeds a context.Context inside it's struct,
contrary to recommended Go practice.

The reason is that there is no unbroken call chain from the time the Request
object is created when the initial HTTP request comes in and when it's 
requested during the Javascript WebSocket HTTP request.

## Security of the WebSocket callback

Each JaWS request gets a unique 64-bit random value assigned to it when you 
create the Request object. This value is written to the HTML output so it
can be read by the Javascript, and used to construct the WebSocket callback
URL.

Once the WebSocket call comes in, the value is consumed by that request,
and is no longer valid until, theoretically, another Request gets the same
random value. And that's fine, since JaWS guarantees that no two Requests
waiting for WebSocket calls can have the same value at the same time.

In addition to this, Requests that are not claimed by a WebSocket call get
cleaned up at regular intervals. By default an unclaimed Request is 
removed after 10 seconds.

In order to guess (and thus hijack) a WebSocket you'd have to make on the
order of 2^63 requests before the genuine request comes in, or 10 seconds
pass assuming you can reliably prevent the genuine WebSocket request.

## Dependencies

We try to minimize dependencies outside of the standard library.

* Depends on https://github.com/nhooyr/websocket for WebSocket functionality.
* Depends on https://github.com/matryer/is for tests.
* Depends on https://github.com/linkdata/deadlock if race detection is enabled.
