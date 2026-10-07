package ui

import (
	"bytes"
	"encoding/json"
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
		alert      string
		hook       bool
		readOnly   bool
		wantPatch  string
		wantChecks int
	}{
		{name: "missing equals", input: "value", wantPatch: `={"value":1}`},
		{name: "invalid path", input: "value..field=2", wantPatch: `={"value":1}`},
		{name: "missing path", input: "missing=2", wantPatch: "missing="},
		{name: "invalid JSON", input: "value={", wantPatch: `={"value":1}`},
		{name: "read-only store", input: "value=2", checkError: ErrJsVarReadOnly, readOnly: true, wantPatch: "value=1"},
		{name: "rejected check", input: "value=2", checkError: errors.New("private check detail"), wantPatch: "value=1", wantChecks: 1},
		{name: "unhandled check", input: "value=2", checkError: fmt.Errorf("private check detail: %w", jaws.ErrEventUnhandled), wantPatch: "value=1", wantChecks: 1},
		{name: "explicit alert", input: "value=2", checkError: errors.New("private check detail"), alert: "Choose a value no greater than 1.", wantPatch: "value=1", wantChecks: 1},
		{name: "hook rejection", input: "value=2", checkError: errors.New("private check detail"), hook: true, wantPatch: "value=1", wantChecks: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				jw, err := jaws.New()
				if err != nil {
					t.Fatal(err)
				}
				logger := new(templateLogger)
				jw.Logger = logger
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
				store.ClientCheck = func(source *jaws.Element, _ *struct {
					Value int `json:"value"`
				}, _ string,
				) error {
					checks++
					if tt.alert != "" {
						source.Request.Alert("warning", tt.alert)
					}
					return tt.checkError
				}
				if tt.readOnly {
					store.ClientCheck = nil
				}
				_, elem, _ := renderTestJsVar(t, tr.Request, store)
				if tt.hook {
					tr.BcastCh <- wire.Message{Dest: store, What: what.Hook, Data: tt.input}
				} else {
					tr.InCh <- wire.WsMsg{Jid: elem.Jid(), What: what.JsVar, Data: tt.input}
				}
				synctest.Wait()
				time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
				synctest.Wait()
				logged := logger.sync(t, jw)
				if len(logged) != 1 {
					t.Fatalf("logged errors = %v, want one rejection", logged)
				}
				if tt.checkError != nil && logged[0].Error() != tt.checkError.Error() {
					t.Fatalf("logged error = %q, want %q", logged[0], tt.checkError)
				}

				var alerts, patches int
				var wantAlerts int
				if tt.alert != "" {
					wantAlerts = 1
				}
				for {
					select {
					case msg, ok := <-tr.OutCh:
						if !ok {
							t.Fatal("request closed before correction")
						}
						switch msg.What {
						case what.Alert:
							alerts++
							if tt.alert == "" || msg.Jid != 0 || msg.Data != "warning\n"+tt.alert {
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
						if alerts != wantAlerts || patches != 1 || checks != tt.wantChecks || state.Value != 1 {
							t.Fatalf("alerts=%d patches=%d checks=%d value=%d", alerts, patches, checks, state.Value)
						}
						return
					}
				}
			})
		})
	}
}

func TestJsVarBrowserNumericConversion(t *testing.T) {
	type stateType struct {
		N uint8 `json:"n"`
		M int   `json:"m"`
	}
	tests := []struct {
		name, input, path string
		want              stateType
	}{
		{name: "uint8 root", input: `={"n":300,"m":0}`, want: stateType{N: 44}},
		{name: "uint8 nested", input: "n=300", path: "n", want: stateType{N: 44}},
		{name: "int root", input: `={"n":0,"m":2.9}`, want: stateType{M: 2}},
		{name: "int nested", input: "m=2.9", path: "m", want: stateType{M: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, rq := newCoreRequest(t)
			var mu sync.RWMutex
			state := stateType{}
			store := newTestJsVarStore(t, jw, "client", &mu, &state)
			var checked stateType
			var checkedPath string
			checks := 0
			store.ClientCheck = func(_ *jaws.Element, next *stateType, path string) error {
				checks++
				checked = *next
				checkedPath = path
				return nil
			}
			binding, elem, _ := renderTestJsVar(t, rq, store)
			if err := binding.JawsInput(elem, tt.input); err != nil {
				t.Fatal(err)
			}
			if checks != 1 || checkedPath != tt.path || checked != tt.want || state != tt.want {
				t.Fatalf("checks=%d path=%q checked=%+v state=%+v; want path=%q state=%+v", checks, checkedPath, checked, state, tt.path, tt.want)
			}
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

func TestJsVarBindingInactive(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	binding := store.Bind()
	elem := rq.NewElement(binding)
	if err := binding.JawsInput(elem, "=2"); err != nil || value != 1 {
		t.Fatalf("input before render: value=%d err=%v", value, err)
	}
	if changed, err := store.SetPath("", 3); err != nil || !changed {
		t.Fatalf("SetPath before render = (%t, %v)", changed, err)
	}
	binding.JawsUpdate(elem)
	if got := binding.lastVersion.Load(); got != 0 {
		t.Fatalf("inactive binding advanced to version %d", got)
	}
}

var errRejectJsVarTestValue = errors.New("reject test value")

type marshalRejectJsVarState struct{ Value int }

func (state marshalRejectJsVarState) MarshalJSON() ([]byte, error) {
	if state.Value < 0 {
		return nil, errRejectJsVarTestValue
	}
	return json.Marshal(struct{ Value int }{Value: state.Value})
}

func TestJsVarProposalEncodingRollback(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := marshalRejectJsVarState{Value: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	checks := 0
	store.ClientCheck = func(*jaws.Element, *marshalRejectJsVarState, string) error {
		checks++
		return nil
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := binding.JawsInput(elem, "Value=-1"); !errors.Is(err, errRejectJsVarTestValue) {
		t.Fatalf("invalid proposal error = %v, want encoding error", err)
	}
	if state.Value != 1 || checks != 0 {
		t.Fatalf("rejected proposal: value=%d checks=%d", state.Value, checks)
	}
	if patches, err := binding.pendingPatches(); err != nil || !reflect.DeepEqual(patches, []string{`={"Value":1}`}) {
		t.Fatalf("rejected proposal correction = (%q, %v)", patches, err)
	}
}

func TestJsVarInvalidServerValueRejectsInputAndCancelsUpdate(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := marshalRejectJsVarState{Value: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	store.ClientCheck = func(*jaws.Element, *marshalRejectJsVarState, string) error { return nil }
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if changed, err := store.SetPath("Value", -1); err != nil || !changed {
		t.Fatalf("SetPath = (%t, %v)", changed, err)
	}
	if err := binding.JawsInput(elem, "Value=2"); !errors.Is(err, errRejectJsVarTestValue) || state.Value != -1 {
		t.Fatalf("invalid current value: state=%d err=%v", state.Value, err)
	}
	binding.JawsUpdate(elem)
	if rq.Context().Err() == nil {
		t.Fatal("invalid server value did not cancel request")
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
