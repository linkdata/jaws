package ui

import (
	"errors"
	"html/template"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

const templateRegisterTemplates = `
{{define "reg-plain"}}plain{{end}}
{{define "reg-page"}}<div id="{{$.RequestWriter.Register $.Dot.Updater}}"></div>{{end}}
`

// registerDot exposes a Template as an Updater to a template action.
type registerDot struct{ updater jaws.Updater }

func (d *registerDot) Updater() jaws.Updater { return d.updater }

func newRegisterRequest(t *testing.T, logger *templateLogger) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, rq := newCoreRequest(t)
	// Logger has to be in place before the Request is created; changing it afterwards is
	// unsupported.
	jw.Logger = logger
	if err := jw.AddTemplateLookuper(template.Must(template.New("reg").Parse(templateRegisterTemplates))); err != nil {
		t.Fatal(err)
	}
	return jw, rq
}

// TestRegister_WrappedTemplateUpdaterReportsUnclaimed checks every reporting path for a
// wrapped Template updater. Register never renders its Element, so the Template has no state
// claim to update against.
func TestRegister_WrappedTemplateUpdaterReportsUnclaimed(t *testing.T) {
	wrapped := NewTemplate("div", "reg-plain", tag.Tag("dot"))

	t.Run("direct call logs", func(t *testing.T) {
		logger := new(templateLogger)
		_, rq := newRegisterRequest(t, logger)
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		if jid := rw.Register(wrapped); !jid.IsValid() {
			t.Fatal("expected a valid Jid even though the update failed")
		}
		if len(logger.errors) != 1 || !errors.Is(logger.errors[0], ErrElementStateUnclaimed) {
			t.Fatalf("logged errors = %v, want one %v", logger.errors, ErrElementStateUnclaimed)
		}
	})

	t.Run("direct call panics without a logger", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		_ = rq.Jaws.AddTemplateLookuper(template.Must(template.New("reg-plain").Parse(`plain`)))
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		// Register runs the update immediately, so MustLog's panic escapes to the caller.
		recovered := func() (recovered any) {
			defer func() { recovered = recover() }()
			rw.Register(wrapped)
			return
		}()
		if err, ok := recovered.(error); !ok || !errors.Is(err, ErrElementStateUnclaimed) {
			t.Fatalf("recovered = %v, want a panic wrapping %v", recovered, ErrElementStateUnclaimed)
		}
	})

	t.Run("template action becomes a render error without a logger", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		_ = rq.Jaws.AddTemplateLookuper(template.Must(template.New("reg").Parse(templateRegisterTemplates)))
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		// html/template recovers a panic raised by a called method and returns it as an
		// execution error, so this surfaces as a render error rather than escaping.
		err := rw.Template("div", "reg-page", &registerDot{updater: wrapped})
		if err == nil {
			t.Fatal("expected a render error from the recovered panic")
		}
		if !strings.Contains(err.Error(), "did not render") {
			t.Fatalf("render error = %v, want it to mention the unclaimed state", err)
		}
	})

	t.Run("template action logs and keeps rendering", func(t *testing.T) {
		logger := new(templateLogger)
		_, rq := newRegisterRequest(t, logger)
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		if err := rw.Template("div", "reg-page", &registerDot{updater: wrapped}); err != nil {
			t.Fatalf("render = %v, want nil: the diagnostic is logged, not fatal", err)
		}
		if !strings.Contains(sb.String(), "</div>") {
			t.Fatalf("rendering did not continue past the diagnostic: %q", sb.String())
		}
		if len(logger.errors) != 1 || !errors.Is(logger.errors[0], ErrElementStateUnclaimed) {
			t.Fatalf("logged errors = %v, want one %v", logger.errors, ErrElementStateUnclaimed)
		}
	})
}

// TestRegister_WrappedTemplateUpdaterOnTheRequestLoop reaches the diagnostic from the
// request loop, which RequestWriter.Register cannot do because it updates immediately: a
// NewRegister child is rendered with a tag, then that tag is dirtied.
func TestRegister_WrappedTemplateUpdaterOnTheRequestLoop(t *testing.T) {
	for _, tt := range []struct {
		name      string
		withLog   bool
		wantAlive bool // does the request loop survive?
	}{
		{"with logger the loop continues", true, true},
		// Request.process recovers the panic and tears the request down; its follow-up Log
		// emits nothing and TestServe's callback sees nil, because process consumed it.
		{"without logger the request is torn down", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			logger := new(templateLogger)
			if tt.withLog {
				jw.Logger = logger
			}
			if err = jw.AddTemplateLookuper(template.Must(template.New("reg-plain").Parse(`plain`))); err != nil {
				t.Fatal(err)
			}
			go jw.Serve()

			tr := jawstest.NewTestRequest(jw, nil)
			t.Cleanup(func() {
				tr.Close()
				<-tr.DoneCh
			})
			<-tr.ReadyCh

			dirty := tag.Tag("registered")
			wrapped := NewTemplate("div", "reg-plain", tag.Tag("dot"))
			// Build the Register Element by hand: RequestWriter.Register would run the
			// failing update immediately, and Register.JawsRender documents that it ignores
			// params, so NewUI would not apply the tag. This leaves the first failing update
			// to the request loop.
			regElem := tr.NewElement(NewRegister(wrapped))
			regElem.Tag(dirty)
			regElem.Freeze()

			tr.BcastCh <- wire.Message{Dest: dirty, What: what.Update}

			if tt.wantAlive {
				// BcastCh preserves send order. Receiving this Alert proves the loop
				// processed the preceding failing update and kept serving afterwards.
				const probe = "still alive"
				tr.BcastCh <- wire.Message{What: what.Alert, Data: probe}
				select {
				case msg, ok := <-tr.OutCh:
					if !ok {
						t.Fatal("the request loop stopped although a logger was configured")
					}
					if msg.Jid != 0 || msg.What != what.Alert || msg.Data != probe {
						t.Fatalf("liveness probe = %+v, want Alert %q", msg, probe)
					}
				case <-tr.DoneCh:
					t.Fatal("the request loop stopped although a logger was configured")
				case <-time.After(2 * time.Second):
					t.Fatal("timeout waiting for the request-loop liveness probe")
				}

				// Join the request-loop goroutine before inspecting its logger writes.
				tr.Close()
				<-tr.DoneCh
				if len(logger.errors) != 1 || !errors.Is(logger.errors[0], ErrElementStateUnclaimed) {
					t.Fatalf("logged errors = %v, want one %v", logger.errors, ErrElementStateUnclaimed)
				}
				return
			}
			select {
			case <-tr.DoneCh:
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for the request loop to tear down")
			}
		})
	}
}

// TestRegister_UnwrappedTemplateUpdaterStaysUsable covers the other half of the narrowing:
// an unwrapped Template remains a valid Register updater because its updates are a
// documented no-op — but only RequestWriter.Register also delivers its event handlers,
// since Register embeds jaws.Updater and therefore promotes no handler methods.
func TestRegister_UnwrappedTemplateUpdaterStaysUsable(t *testing.T) {
	dot := &clickCountingDot{}
	unwrapped := NewTemplate("", "reg-plain", dot)

	// The UI value itself delegates nothing: Register embeds jaws.Updater, whose method set
	// is JawsUpdate alone, so a Template's handler methods are never promoted onto it.
	if _, ok := any(NewRegister(unwrapped)).(jaws.ClickHandler); ok {
		t.Fatal("Register promoted a ClickHandler; it embeds only jaws.Updater")
	}

	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	logger := new(templateLogger)
	jw.Logger = logger
	if err = jw.AddTemplateLookuper(template.Must(template.New("reg-plain").Parse(`plain`))); err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	var sb strings.Builder
	rw := RequestWriter{Request: tr.Request, Writer: &sb}
	jid := rw.Register(unwrapped)
	if len(logger.errors) != 0 {
		t.Fatalf("logged errors = %v, want none for an unwrapped Template", logger.errors)
	}

	// RequestWriter.Register added the concrete updater to the Element's handler list, so a
	// browser event reaches the Template and through it the Dot.
	// Click data is "X Y kstate name"; a bare name does not parse.
	tr.InCh <- wire.WsMsg{Jid: jid, What: what.Click, Data: "1 2 0 btn"}
	deadline := time.Now().Add(2 * time.Second)
	for dot.clicks.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := dot.clicks.Load(); got != 1 {
		t.Fatalf("clicks delivered through the handler list = %d, want 1", got)
	}
}

// clickCountingDot counts clicks delivered through a Template's event delegation.
type clickCountingDot struct{ clicks atomic.Int32 }

func (d *clickCountingDot) JawsClick(*jaws.Element, jaws.Click) error {
	d.clicks.Add(1)
	return nil
}
