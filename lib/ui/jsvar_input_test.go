package ui

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestJsVarRejectedBrowserInput(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		checkError error
		wantPatch  string
		wantChecks int
	}{
		{name: "missing equals", input: "value", wantPatch: `={"value":1}`},
		{name: "invalid path", input: "value..field=2", wantPatch: `={"value":1}`},
		{name: "missing path", input: "missing=2", wantPatch: "missing="},
		{name: "invalid JSON", input: "value={", wantPatch: `={"value":1}`},
		{name: "rejected check", input: "value=2", checkError: errors.New("private check detail"), wantPatch: "value=1", wantChecks: 1},
		{name: "unhandled check", input: "value=2", checkError: fmt.Errorf("private check detail: %w", jaws.ErrEventUnhandled), wantPatch: "value=1", wantChecks: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				jw, err := jaws.New()
				if err != nil {
					t.Fatal(err)
				}
				go jw.Serve()
				tr := jawstest.NewTestRequest(jw, nil)
				<-tr.ReadyCh
				defer func() {
					tr.Close()
					jw.Close()
					synctest.Wait()
				}()

				var mu sync.RWMutex
				state := struct {
					Value int `json:"value"`
				}{Value: 1}
				store := newTestJsVarStore(t, jw, "client", &mu, &state)
				checks := 0
				store.ClientCheck = func(*jaws.Element, *struct {
					Value int `json:"value"`
				}, string,
				) error {
					checks++
					return tt.checkError
				}
				_, elem, _ := renderTestJsVar(t, tr.Request, store)
				tr.InCh <- wire.WsMsg{Jid: elem.Jid(), What: what.JsVar, Data: tt.input}
				synctest.Wait()
				time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
				synctest.Wait()

				var alerts, patches int
				for {
					select {
					case msg, ok := <-tr.OutCh:
						if !ok {
							t.Fatal("request closed before correction")
						}
						switch msg.What {
						case what.Alert:
							alerts++
							if msg.Jid != 0 || msg.Data != "danger\ninvalid JsVar update" {
								t.Fatalf("browser alert = %#v", msg)
							}
						case what.JsVar:
							patches++
							if msg.Jid != elem.Jid() || msg.Data != tt.wantPatch {
								t.Fatalf("correction = %#v, want %q", msg, tt.wantPatch)
							}
						default:
							t.Fatalf("unexpected browser message: %#v", msg)
						}
					default:
						if alerts != 1 || patches != 1 || checks != tt.wantChecks || state.Value != 1 {
							t.Fatalf("alerts=%d patches=%d checks=%d value=%d", alerts, patches, checks, state.Value)
						}
						return
					}
				}
			})
		})
	}
}

func TestErrJsVarClientWriteError(t *testing.T) {
	cause := errors.New("private check detail")
	err := errJsVarClientWrite{cause}
	if got := err.Error(); got != cause.Error() {
		t.Fatalf("server error = %q, want %q", got, cause)
	}
}

func TestValidateJsVarPathLengthAndUTF8(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "maximum length", path: strings.Repeat("a", maxJsVarPathBytes)},
		{name: "too long", path: strings.Repeat("a", maxJsVarPathBytes+1), wantErr: true},
		{name: "invalid UTF8", path: string([]byte{0xff}), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJsVarPath(tt.path)
			if got := errors.Is(err, ErrIllegalJsVarPath); got != tt.wantErr {
				t.Fatalf("validateJsVarPath() error = %v, want illegal path %v", err, tt.wantErr)
			}
		})
	}
}

func TestVisibleJsVarChangeSlice(t *testing.T) {
	before := []any{map[string]any{"value": 1}, 2}
	tests := []struct {
		name  string
		after any
		path  string
		want  bool
	}{
		{name: "selected element changed", after: []any{map[string]any{"value": 3}, 2}, path: "0.value", want: true},
		{name: "selected element unchanged", after: before, path: "0.value"},
		{name: "other element changed", after: []any{map[string]any{"value": 3}, 4}, path: "0.value"},
		{name: "length changed", after: []any{map[string]any{"value": 3}}, path: "0.value"},
		{name: "different shape", after: map[string]any{"0": 3}, path: "0.value"},
		{name: "non-numeric index", after: []any{map[string]any{"value": 3}, 2}, path: "first.value"},
		{name: "negative index", after: []any{map[string]any{"value": 3}, 2}, path: "-1.value"},
		{name: "out of range", after: []any{map[string]any{"value": 3}, 2}, path: "2.value"},
		{name: "noncanonical index", after: []any{map[string]any{"value": 3}, 2}, path: "00.value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visibleJsVarChange(before, tt.after, tt.path); got != tt.want {
				t.Fatalf("visibleJsVarChange(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
	if visibleJsVarChange(1, 2, "value") {
		t.Fatal("scalar value accepted a child path")
	}
}

func TestPlainJsVarTypeStructRules(t *testing.T) {
	type embedded struct{ Value int }
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "simple struct", value: struct {
			Value int `json:"value"`
		}{}, want: true},
		{name: "anonymous field", value: struct{ embedded }{}},
		{name: "unexported field ignored", value: struct {
			Value  int `json:"value"`
			hidden []int
		}{}, want: true},
		{name: "ignored field", value: struct {
			Hidden []int `json:"-"`
		}{}, want: true},
		{name: "invalid name", value: struct {
			Value int `json:"bad-name"`
		}{}},
		{name: "duplicate name", value: reflect.New(reflect.StructOf([]reflect.StructField{
			{Name: "First", Type: reflect.TypeFor[int](), Tag: `json:"value"`},
			{Name: "Second", Type: reflect.TypeFor[int](), Tag: `json:"value"`},
		})).Elem().Interface()},
		{name: "scalar string option", value: struct {
			Value int `json:"value,string"`
		}{}, want: true},
		{name: "complex string option", value: reflect.New(reflect.StructOf([]reflect.StructField{
			{Name: "Value", Type: reflect.TypeFor[[2]int](), Tag: `json:"value,string"`},
		})).Elem().Interface()},
		{name: "unknown option", value: struct {
			Value int `json:"value,unknown"`
		}{}},
		{name: "omitempty option", value: struct {
			Value int `json:"value,omitempty"`
		}{}},
		{name: "slice field", value: struct {
			Values []int `json:"values"`
		}{}},
		{name: "array field", value: struct {
			Values [2]int `json:"values"`
		}{}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainJsVarType(reflect.TypeOf(tt.value)); got != tt.want {
				t.Fatalf("plainJsVarType(%T) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestJsVarBindingSecondRender(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	_, elem, _ := renderTestJsVar(t, rq, store)
	var output bytes.Buffer
	if err := elem.JawsRender(&output, nil); !errors.Is(err, ErrJsVarBindingUsed) {
		t.Fatalf("second render error = %v, want ErrJsVarBindingUsed", err)
	}
	if output.Len() != 0 {
		t.Fatalf("second render output = %q", output.String())
	}
}
