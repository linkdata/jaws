package jaws

import (
	"html/template"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type testJawsClick struct {
	clickCh chan string
	*testSetter[string]
}

func (tjc *testJawsClick) JawsClick(elem *Element, click Click) (err error) {
	if err = tjc.Err(); err == nil {
		tjc.clickCh <- click.Name
	}
	return
}

var _ ClickHandler = (*testJawsClick)(nil)

type testJawsContextMenu struct {
	clickCh chan Click
	*testSetter[Click]
}

func (tjc *testJawsContextMenu) JawsContextMenu(elem *Element, click Click) (err error) {
	if err = tjc.Err(); err == nil {
		tjc.clickCh <- click
	}
	return
}

var _ ContextMenuHandler = (*testJawsContextMenu)(nil)

type testJawsInitialHTMLAttr struct{}

func (testJawsInitialHTMLAttr) JawsInitialHTMLAttr(elem *Element) template.HTMLAttr {
	return `data-test="1"`
}

var _ InitialHTMLAttrHandler = testJawsInitialHTMLAttr{}

func Test_clickHandlerWrapper_Dispatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		tjc := &testJawsClick{
			clickCh:    make(chan string),
			testSetter: newTestSetter(""),
		}

		want := `<div id="Jid.1">inner</div>`
		if err := rq.UI(testDivWidget{inner: template.HTML("inner")}, tjc); err != nil {
			t.Fatal(err)
		}
		if got := rq.BodyString(); got != want {
			t.Errorf("Request.UI(NewDiv()) = %q, want %q", got, want)
		}

		// An Input message to a div (which has no input handler) must produce no
		// output. synctest.Wait blocks until the process loop has fully handled the
		// message, so the negative assertion is not vacuous: a bare default: select
		// would short-circuit before the async process goroutine could react.
		rq.InCh <- wire.WsMsg{Data: "text", Jid: 1, What: what.Input}
		synctest.Wait()
		select {
		case s := <-rq.OutCh:
			t.Errorf("unexpected output for Input: %q", s.Format())
		default:
		}

		// A malformed click ("adam", missing coordinates) must be ignored before the
		// handler is invoked.
		rq.InCh <- wire.WsMsg{Data: "adam", Jid: 1, What: what.Click}
		synctest.Wait()
		select {
		case name := <-tjc.clickCh:
			t.Fatalf("malformed click should be ignored, got %q", name)
		default:
		}

		// A well-formed click dispatches to the handler.
		rq.InCh <- wire.WsMsg{Data: "1 2 0 adam", Jid: 1, What: what.Click}
		synctest.Wait()
		select {
		case name := <-tjc.clickCh:
			if name != "adam" {
				t.Error(name)
			}
		default:
			t.Fatal("expected click to dispatch to handler")
		}
	})
}

func Test_defaultAuth(t *testing.T) {
	a := DefaultAuth{}
	if a.Data() != nil {
		t.Fatal()
	}
	if a.Email() != "" {
		t.Fatal()
	}
	if a.IsAdmin() != true {
		t.Fatal()
	}
}

func TestJaws_DefaultAuthReturnsSharedInstance(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	// A shared instance keeps the embedded sync.Once effective across renders so
	// the fail-open warning is logged once per Jaws, not once per template render.
	a := jw.DefaultAuth()
	if a == nil {
		t.Fatal("DefaultAuth returned nil")
	}
	if jw.DefaultAuth() != a {
		t.Fatal("DefaultAuth must return the same shared instance")
	}
}

type reentrantDefaultAuthLogger struct {
	jw        *Jaws
	warnCalls int
	reentered bool
}

func (*reentrantDefaultAuthLogger) Info(string, ...any)  {}
func (*reentrantDefaultAuthLogger) Error(string, ...any) {}

func (logger *reentrantDefaultAuthLogger) Warn(string, ...any) {
	logger.warnCalls++
	logger.reentered = logger.jw.DefaultAuth().IsAdmin()
}

func TestDefaultAuth_IsAdminLoggerCanReenter(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	logger := &reentrantDefaultAuthLogger{jw: jw}
	jw.Logger = logger

	done := make(chan bool, 1)
	go func() {
		done <- jw.DefaultAuth().IsAdmin()
	}()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case isAdmin := <-done:
		if !isAdmin {
			t.Error("DefaultAuth.IsAdmin returned false")
		}
	case <-timer.C:
		t.Fatal("DefaultAuth.IsAdmin deadlocked when Logger.Warn re-entered it")
	case <-t.Context().Done():
		t.Fatalf("test context ended while Logger.Warn re-entered DefaultAuth.IsAdmin: %v", t.Context().Err())
	}
	if logger.warnCalls != 1 {
		t.Errorf("Logger.Warn calls = %d, want 1", logger.warnCalls)
	}
	if !logger.reentered {
		t.Error("Logger.Warn re-entry returned false")
	}
}

func Test_InitialHTMLAttrHandler_IgnoredByDispatch(t *testing.T) {
	if err := callEventHandler(testJawsInitialHTMLAttr{}, nil, what.Input, "ignored"); err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled, got %v", err)
	}
}
