package ui_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
)

type rejectingInputBinder[T comparable] struct {
	bind.Binder[T]
	err error
}

func (b rejectingInputBinder[T]) JawsSet(*jaws.Element, T) error { return b.err }

func newRejectingInputBinder[T comparable](value T, err error) rejectingInputBinder[T] {
	var mu sync.Mutex
	return rejectingInputBinder[T]{bind.New(&mu, &value), err}
}

type normalizingInputBinder struct{ bind.Binder[string] }

func (b normalizingInputBinder) JawsSet(elem *jaws.Element, value string) error {
	return b.Binder.JawsSet(elem, strings.TrimSpace(value))
}

func newNormalizingInputBinder(value string) normalizingInputBinder {
	var mu sync.Mutex
	return normalizingInputBinder{bind.New(&mu, &value)}
}

type externalEmailInput struct{ ui.InputText }

// JawsRender renders an email input through the embedded string base.
func (u *externalEmailInput) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderInput(elem, w, "email", params...)
}

type externalCheckboxInput struct{ ui.InputBool }

// JawsRender renders a checkbox through the embedded boolean base.
func (u *externalCheckboxInput) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderInput(elem, w, "checkbox", params...)
}

type externalDateInput struct{ ui.InputDate }

// JawsRender renders a date input through the embedded date base.
func (u *externalDateInput) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderInput(elem, w, "date", params...)
}

type externalInputWidget interface {
	jaws.UI
	jaws.InputHandler
}

func TestExternalInputRenderReconcilesInput(t *testing.T) {
	rejected := errors.New("rejected")
	tests := []struct {
		name     string
		widget   externalInputWidget
		htmlType string
		initial  string
		input    string
		want     string
		wantErr  error
	}{
		{
			name:     "string",
			widget:   &externalEmailInput{ui.InputText{Setter: newRejectingInputBinder("server", rejected)}},
			htmlType: "email",
			initial:  `value="server"`,
			input:    "typed",
			want:     "server",
			wantErr:  rejected,
		},
		{
			name:     "normalized string",
			widget:   &externalEmailInput{ui.InputText{Setter: newNormalizingInputBinder("server")}},
			htmlType: "email",
			initial:  `value="server"`,
			input:    " next ",
			want:     "next",
		},
		{
			name:     "bool",
			widget:   &externalCheckboxInput{ui.InputBool{Setter: newRejectingInputBinder(true, rejected)}},
			htmlType: "checkbox",
			initial:  " checked",
			input:    "false",
			want:     "true",
			wantErr:  rejected,
		},
		{
			name:     "date",
			widget:   &externalDateInput{ui.InputDate{Setter: newRejectingInputBinder(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), rejected)}},
			htmlType: "date",
			initial:  `value="2024-01-02"`,
			input:    "2025-03-04",
			want:     "2024-01-02",
			wantErr:  rejected,
		},
		{
			name:     "date clear",
			widget:   &externalDateInput{ui.InputDate{Setter: newRejectingInputBinder(time.Time{}, jaws.ErrValueUnchanged)}},
			htmlType: "date",
			initial:  `value="0001-01-01"`,
			input:    "",
			want:     "0001-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			go jw.Serve()

			tr := jawstest.NewTestRequest(jw, nil)
			defer tr.Close()
			<-tr.ReadyCh

			elem := tr.NewElement(tt.widget)
			var html strings.Builder
			if err := elem.JawsRender(&html, nil); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(html.String(), `type="`+tt.htmlType+`"`) {
				t.Fatalf("rendered input = %q, want type %q", html.String(), tt.htmlType)
			}
			if !strings.Contains(html.String(), tt.initial) {
				t.Fatalf("rendered input = %q, want %q", html.String(), tt.initial)
			}
			if err := tt.widget.JawsInput(elem, tt.input); !errors.Is(err, tt.wantErr) {
				t.Fatalf("JawsInput error = %v, want %v", err, tt.wantErr)
			}

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			select {
			case msg := <-tr.OutCh:
				if msg.What != what.Value || msg.Jid != elem.Jid() || msg.Data != tt.want {
					t.Fatalf("update = %#v, want Value %q for %v", msg, tt.want, elem.Jid())
				}
			case <-ctx.Done():
				t.Fatal("no corrective update received")
			}
		})
	}
}
