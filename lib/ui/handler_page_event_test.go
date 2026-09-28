package ui

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type pageEventAuth struct{}

func (pageEventAuth) Data() map[string]any { return nil }
func (pageEventAuth) Email() string        { return "user@example.test" }
func (pageEventAuth) IsAdmin() bool        { return false }

type pageEventDot struct{ called chan string }

func (dot *pageEventDot) JawsClick(_ *jaws.Element, click jaws.Click) error {
	dot.called <- "click:" + click.Name
	return nil
}

func (dot *pageEventDot) JawsContextMenu(_ *jaws.Element, click jaws.Click) error {
	dot.called <- "context:" + click.Name
	return nil
}

func (dot *pageEventDot) JawsInput(_ *jaws.Element, value string) error {
	dot.called <- "input:" + value
	return nil
}

func TestHandlerPageElementRejectsPeerEvents(t *testing.T) {
	dot := &pageEventDot{called: make(chan string, 8)}
	ping := &pageEventDot{called: make(chan string, 1)}
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}{{$.Button "ping" (pingDot)}}{{if .Auth.IsAdmin}}{{$.Button "wipe" .Dot}}{{end}}`, dot, template.FuncMap{
		"pingDot": func() any { return ping },
	})
	ts.jaws.MakeAuth = func(*jaws.Request) jaws.Auth { return pageEventAuth{} }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, err := ts.get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "wipe") {
		t.Fatalf("admin button rendered for non-admin: %q", body)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	if rq.GetElementByJid(2) == nil || rq.GetElementByJid(3) != nil {
		t.Fatal("expected only page and ping Elements")
	}
	conn, err := ts.dial(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	defer closeHandlerWebSocket(t, conn)
	for _, msg := range []wire.WsMsg{
		{Jid: 1, What: what.Click, Data: "0 0 0 wipe"},
		{What: what.Click, Data: "0 0 0 wipe\tJid.1"},
		{Jid: 1, What: what.ContextMenu, Data: "0 0 0 wipe"},
		{Jid: 1, What: what.Input, Data: "wipe"},
		{Jid: 1, What: what.JsVar, Data: "wipe=1"},
		{Jid: 2, What: what.Click, Data: "0 0 0 ping"},
	} {
		if err := conn.Write(ctx, websocket.MessageText, msg.Append(nil)); err != nil {
			t.Fatal(err)
		}
	}
	if got := receiveHandlerWebSocketValue(t, ctx, ping.called, "ping click"); got != "click:ping" {
		t.Fatalf("ping click = %q", got)
	}
	select {
	case got := <-dot.called:
		t.Fatalf("page Element forwarded peer event: %q", got)
	default:
	}
}

func TestHandlerPageElementRejectsBinderHook(t *testing.T) {
	var mu deadlock.Mutex
	var value string
	called := make(chan string, 2)
	ping := &pageEventDot{called: make(chan string, 1)}
	dot := bind.New(&mu, &value).Clicked(func(_ bind.Binder[string], _ *jaws.Element, click jaws.Click) error {
		called <- click.Name
		return nil
	})
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}{{$.Button "ping" (pingDot)}}{{if .Auth.IsAdmin}}{{$.Button "wipe" .Dot}}{{end}}`, dot, template.FuncMap{
		"pingDot": func() any { return ping },
	})
	ts.jaws.MakeAuth = func(*jaws.Request) jaws.Auth { return pageEventAuth{} }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := ts.get(ctx); err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	conn, err := ts.dial(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	defer closeHandlerWebSocket(t, conn)
	for _, msg := range []wire.WsMsg{
		{Jid: 1, What: what.Click, Data: "0 0 0 wipe"},
		{Jid: 2, What: what.Click, Data: "0 0 0 ping"},
	} {
		if err := conn.Write(ctx, websocket.MessageText, msg.Append(nil)); err != nil {
			t.Fatal(err)
		}
	}
	if got := receiveHandlerWebSocketValue(t, ctx, ping.called, "ping click"); got != "click:ping" {
		t.Fatalf("ping click = %q", got)
	}
	select {
	case got := <-called:
		t.Fatalf("page Element invoked Binder hook: %q", got)
	default:
	}
}
