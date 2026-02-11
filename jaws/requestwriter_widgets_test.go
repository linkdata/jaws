package jaws

import (
	"io"
	"testing"
	"time"
)

type rwNoopUI struct{}

func (rwNoopUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (rwNoopUI) JawsUpdate(*Element)                         {}

type rwTestContainer struct{}

func (rwTestContainer) JawsContains(*Element) []UI { return nil }

func testRequestWriterWidgetFactory() RequestWriterWidgetFactory {
	return RequestWriterWidgetFactory{
		A:         func(HTMLGetter) UI { return rwNoopUI{} },
		Button:    func(HTMLGetter) UI { return rwNoopUI{} },
		Checkbox:  func(Setter[bool]) UI { return rwNoopUI{} },
		Container: func(string, Container) UI { return rwNoopUI{} },
		Date:      func(Setter[time.Time]) UI { return rwNoopUI{} },
		Div:       func(HTMLGetter) UI { return rwNoopUI{} },
		Img:       func(Getter[string]) UI { return rwNoopUI{} },
		Label:     func(HTMLGetter) UI { return rwNoopUI{} },
		Li:        func(HTMLGetter) UI { return rwNoopUI{} },
		Number:    func(Setter[float64]) UI { return rwNoopUI{} },
		Password:  func(Setter[string]) UI { return rwNoopUI{} },
		Radio:     func(Setter[bool]) UI { return rwNoopUI{} },
		Range:     func(Setter[float64]) UI { return rwNoopUI{} },
		Select:    func(SelectHandler) UI { return rwNoopUI{} },
		Span:      func(HTMLGetter) UI { return rwNoopUI{} },
		Tbody:     func(Container) UI { return rwNoopUI{} },
		Td:        func(HTMLGetter) UI { return rwNoopUI{} },
		Text:      func(Setter[string]) UI { return rwNoopUI{} },
		Textarea:  func(Setter[string]) UI { return rwNoopUI{} },
		Tr:        func(HTMLGetter) UI { return rwNoopUI{} },
	}
}

func TestRequestWriterWidgets_requiresRegistration(t *testing.T) {
	old := requestWriterWidgets
	requestWriterWidgets = RequestWriterWidgetFactory{}
	defer func() { requestWriterWidgets = old }()

	rw := RequestWriter{}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = rw.Span("x")
}

func TestRequestWriterWidgets_methods(t *testing.T) {
	old := requestWriterWidgets
	defer func() { requestWriterWidgets = old }()

	RegisterRequestWriterWidgets(testRequestWriterWidgetFactory())

	tr := newTestRequest(t)
	defer tr.Close()
	rw := tr.RequestWriter

	if err := rw.A("a"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Button("button"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Checkbox(true); err != nil {
		t.Fatal(err)
	}
	if err := rw.Container("div", rwTestContainer{}); err != nil {
		t.Fatal(err)
	}
	if err := rw.Date(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := rw.Div("div"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Img("img"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Label("label"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Li("li"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Number(float64(1.2)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Password("pw"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Radio(true); err != nil {
		t.Fatal(err)
	}
	if err := rw.Range(float32(1.5)); err != nil {
		t.Fatal(err)
	}
	if err := rw.Select(NewNamedBoolArray()); err != nil {
		t.Fatal(err)
	}
	if err := rw.Span("span"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Tbody(rwTestContainer{}); err != nil {
		t.Fatal(err)
	}
	if err := rw.Td("td"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Text("txt"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Textarea("ta"); err != nil {
		t.Fatal(err)
	}
	if err := rw.Tr("tr"); err != nil {
		t.Fatal(err)
	}
}
