package ui

import (
	"errors"
	"html/template"
	"strings"
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
	// The Logger has to be in place before the Request is created; changing it afterwards
	// is unsupported, so it goes in through the configure hook.
	jw, rq := newConfiguredCoreRequest(t, withLogger(logger))
	if err := jw.AddTemplateLookuper(template.Must(template.New("reg").Parse(templateRegisterTemplates))); err != nil {
		t.Fatal(err)
	}
	return jw, rq
}

// TestRegister_TemplateUpdaterReportsUnclaimed checks every reporting path for a
// Template updater. RequestWriter.Register never calls the Template's renderer,
// so the Template has no state claim to update against.
func TestRegister_TemplateUpdaterReportsUnclaimed(t *testing.T) {
	tmpl := NewTemplate("div", "reg-plain", tag.Tag("dot"))

	t.Run("direct call logs", func(t *testing.T) {
		logger := new(templateLogger)
		jw, rq := newRegisterRequest(t, logger)
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		if jid := rw.Register(tmpl); !jid.IsValid() {
			t.Fatal("expected a valid Jid even though the update failed")
		}
		logged := logger.sync(t, jw)
		if len(logged) != 1 || !errors.Is(logged[0], ErrElementStateUnclaimed) {
			t.Fatalf("logged errors = %v, want one %v", logged, ErrElementStateUnclaimed)
		}
	})

	t.Run("direct call panics without a logger", func(t *testing.T) {
		jw, rq := newCoreRequest(t)
		if err := jw.AddTemplateLookuper(template.Must(template.New("reg-plain").Parse(`plain`))); err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		// Register runs the update immediately, so MustLog's panic escapes to the caller.
		recovered := func() (recovered any) {
			defer func() { recovered = recover() }()
			rw.Register(tmpl)
			return
		}()
		if err, ok := recovered.(error); !ok || !errors.Is(err, ErrElementStateUnclaimed) {
			t.Fatalf("recovered = %v, want a panic wrapping %v", recovered, ErrElementStateUnclaimed)
		}
	})

	t.Run("template action becomes a render error without a logger", func(t *testing.T) {
		jw, rq := newCoreRequest(t)
		if err := jw.AddTemplateLookuper(template.Must(template.New("reg").Parse(templateRegisterTemplates))); err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		// html/template recovers a panic raised by a called method and returns it as an
		// execution error, so this surfaces as a render error rather than escaping. The
		// wrapping preserves the sentinel, so match on that rather than on the text.
		err := rw.Template("div", "reg-page", &registerDot{updater: tmpl})
		if err == nil {
			t.Fatal("expected a render error from the recovered panic")
		}
		if !errors.Is(err, ErrElementStateUnclaimed) {
			t.Fatalf("render error = %v, want it to wrap %v", err, ErrElementStateUnclaimed)
		}
	})

	t.Run("template action logs and keeps rendering", func(t *testing.T) {
		logger := new(templateLogger)
		jw, rq := newRegisterRequest(t, logger)
		var sb strings.Builder
		rw := RequestWriter{Request: rq, Writer: &sb}

		if err := rw.Template("div", "reg-page", &registerDot{updater: tmpl}); err != nil {
			t.Fatalf("render = %v, want nil: the diagnostic is logged, not fatal", err)
		}
		if !strings.Contains(sb.String(), "</div>") {
			t.Fatalf("rendering did not continue past the diagnostic: %q", sb.String())
		}
		logged := logger.sync(t, jw)
		if len(logged) != 1 || !errors.Is(logged[0], ErrElementStateUnclaimed) {
			t.Fatalf("logged errors = %v, want one %v", logged, ErrElementStateUnclaimed)
		}
	})
}

// TestRegister_TemplateUpdaterOnTheRequestLoop reaches the diagnostic from the
// request loop, which RequestWriter.Register cannot do because it updates immediately: a
// registerUI child is rendered with a tag, then that tag is dirtied.
func TestRegister_TemplateUpdaterOnTheRequestLoop(t *testing.T) {
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
			tmpl := NewTemplate("div", "reg-plain", tag.Tag("dot"))
			// Build the registered Element by hand: RequestWriter.Register would run the
			// failing update immediately, and registerUI.JawsRender ignores params, so
			// NewUI would not apply the tag. This leaves the first failing update to the
			// request loop.
			regElem := tr.NewElement(registerUI{Updater: tmpl})
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

				// Join the request-loop goroutine, then drain the asynchronous logger.
				tr.Close()
				<-tr.DoneCh
				logged := logger.sync(t, jw)
				if len(logged) != 1 || !errors.Is(logged[0], ErrElementStateUnclaimed) {
					t.Fatalf("logged errors = %v, want one %v", logged, ErrElementStateUnclaimed)
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
