package jaws

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUiWrapContainer_RenderError(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	// Create a test UI that returns an error when rendering
	renderErr := errors.New("render error")
	errorUI := &testUi{
		renderFn: func(e *Element, w io.Writer, params []any) error {
			// Write partial content before returning error
			_, _ = w.Write([]byte("partial"))
			return renderErr
		},
	}

	// Create a container with a successful UI followed by an error UI
	tc := &testContainer{
		contents: []UI{
			NewUiSpan(testHTMLGetter("first")),
			errorUI,
			NewUiSpan(testHTMLGetter("third")), // This should not be rendered
		},
	}

	ui := NewUiContainer("div", tc)
	elem := rq.NewElement(ui)
	var sb strings.Builder

	// Render should return the error
	err := ui.JawsRender(elem, &sb, nil)
	if err == nil {
		t.Fatal("expected error from JawsRender, got nil")
	}
	if !errors.Is(err, renderErr) {
		t.Errorf("expected error %v, got %v", renderErr, err)
	}

	// Verify the output contains the opening tag and first successful element
	output := sb.String()
	if !strings.Contains(output, `<div id="Jid.1">`) {
		t.Errorf("expected opening tag in output, got: %s", output)
	}
	if !strings.Contains(output, `<span id="Jid.2">first</span>`) {
		t.Errorf("expected first span in output, got: %s", output)
	}
	if !strings.Contains(output, "partial") {
		t.Errorf("expected partial content from error UI, got: %s", output)
	}

	// The third element should not be rendered
	if strings.Contains(output, "third") {
		t.Errorf("third element should not be rendered, got: %s", output)
	}

	// Verify that ui.contents contains only the successfully rendered elements
	ui.mu.Lock()
	contentsLen := len(ui.contents)
	ui.mu.Unlock()

	if contentsLen != 1 {
		t.Errorf("expected ui.contents to have 1 element (only first successful), got %d", contentsLen)
	}
}

func TestUiWrapContainer_RenderErrorFirstElement(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	renderErr := errors.New("immediate error")
	errorUI := &testUi{
		renderFn: func(e *Element, w io.Writer, params []any) error {
			return renderErr
		},
	}

	// Container with only an error-producing UI as the first element
	tc := &testContainer{
		contents: []UI{errorUI},
	}

	ui := NewUiContainer("div", tc)
	elem := rq.NewElement(ui)
	var sb strings.Builder

	// Render should return the error immediately
	err := ui.JawsRender(elem, &sb, nil)
	if err == nil {
		t.Fatal("expected error from JawsRender, got nil")
	}
	if !errors.Is(err, renderErr) {
		t.Errorf("expected error %v, got %v", renderErr, err)
	}

	// Verify ui.contents is empty since no elements were successfully rendered
	ui.mu.Lock()
	contentsLen := len(ui.contents)
	ui.mu.Unlock()

	if contentsLen != 0 {
		t.Errorf("expected ui.contents to be empty, got %d elements", contentsLen)
	}
}

func TestUiWrapContainer_RenderAllSuccess(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	// Container with all successful renders
	tc := &testContainer{
		contents: []UI{
			NewUiSpan(testHTMLGetter("first")),
			NewUiSpan(testHTMLGetter("second")),
			NewUiSpan(testHTMLGetter("third")),
		},
	}

	ui := NewUiContainer("div", tc)
	elem := rq.NewElement(ui)
	var sb strings.Builder

	// Render should succeed
	err := ui.JawsRender(elem, &sb, nil)
	if err != nil {
		t.Fatalf("unexpected error from JawsRender: %v", err)
	}

	// Verify all elements are rendered
	output := sb.String()
	expected := `<div id="Jid.1"><span id="Jid.2">first</span><span id="Jid.3">second</span><span id="Jid.4">third</span></div>`
	if output != expected {
		t.Errorf("output mismatch\nwant: %s\ngot:  %s", expected, output)
	}

	// Verify ui.contents has all three elements
	ui.mu.Lock()
	contentsLen := len(ui.contents)
	ui.mu.Unlock()

	if contentsLen != 3 {
		t.Errorf("expected ui.contents to have 3 elements, got %d", contentsLen)
	}
}
