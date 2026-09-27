package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jq"
)

func newTestJsVarStore[T any](t *testing.T, jw *jaws.Jaws, name string, mu *sync.RWMutex, value *T) *JsVarStore[T] {
	t.Helper()
	store, err := NewJsVarStore(jw, name, mu, value)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func renderTestJsVar[T any](t *testing.T, rq *jaws.Request, store *JsVarStore[T]) (*JsVarBinding[T], *jaws.Element, string) {
	t.Helper()
	binding := store.Bind()
	elem := rq.NewElement(binding)
	var out bytes.Buffer
	if err := elem.JawsRender(&out, nil); err != nil {
		t.Fatal(err)
	}
	return binding, elem, out.String()
}

func TestJsVarStorePolicyAndCorrection(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Value int `json:"value"`
	}{Value: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	binding, elem, html := renderTestJsVar(t, rq, store)
	if !strings.Contains(html, `data-jawsstore="client"`) || !strings.Contains(html, `data-jawsdata=`) {
		t.Fatalf("missing store marker or snapshot: %q", html)
	}
	if err := binding.JawsInput(elem, "value=2"); !errors.Is(err, ErrJsVarReadOnly) || state.Value != 1 {
		t.Fatalf("nil check: state=%d err=%v", state.Value, err)
	}
	checks := 0
	store.ClientCheck = func(_ *jaws.Element, next *struct {
		Value int `json:"value"`
	}, path string,
	) error {
		checks++
		if path != "value" || next.Value > 2 {
			return errors.New("rejected")
		}
		return nil
	}
	if err := binding.JawsInput(elem, "value=2"); err != nil || state.Value != 2 || checks != 1 {
		t.Fatalf("accepted proposal: state=%d checks=%d err=%v", state.Value, checks, err)
	}
	if err := binding.JawsInput(elem, "value=3"); err == nil || state.Value != 2 || checks != 2 {
		t.Fatalf("rejected proposal: state=%d checks=%d err=%v", state.Value, checks, err)
	}
	if err := binding.JawsInput(elem, "value=2"); err != nil || checks != 2 {
		t.Fatalf("unchanged proposal: checks=%d err=%v", checks, err)
	}
	if err := binding.JawsInput(elem, "Value=4"); !errors.Is(err, jq.ErrPathNotFound) || state.Value != 2 {
		t.Fatalf("noncanonical field spelling: state=%d err=%v", state.Value, err)
	}
	if got := store.projectPatch("value", []byte(`{"value":2}`)); got != "value=2" {
		t.Fatalf("canonical partial = %q", got)
	}
}

func TestJsVarStoreJSONSizeRollback(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Text string `json:"text"`
	}{Text: "ok"}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	store.ClientCheck = JSONSizeCheck[struct {
		Text string `json:"text"`
	}](16)
	binding, elem, _ := renderTestJsVar(t, rq, store)
	err := binding.JawsInput(elem, `text="long long long long"`)
	if !errors.Is(err, ErrJsVarTooLarge) || state.Text != "ok" {
		t.Fatalf("oversize proposal: state=%q err=%v", state.Text, err)
	}
	if rq.Context().Err() == nil {
		t.Fatal("oversize proposal did not cancel its Request")
	}
}

func TestJsVarStoreCheckPanicCorrectsSource(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	t.Cleanup(jw.Close)
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	var mu sync.RWMutex
	state := struct {
		Value int `json:"value"`
	}{Value: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	store.ClientCheck = func(*jaws.Element, *struct {
		Value int `json:"value"`
	}, string,
	) error {
		panic("check")
	}
	binding, elem, _ := renderTestJsVar(t, tr.Request, store)
	if err := jaws.CallEventHandlers(binding, elem, what.Proposal, "value=2"); err == nil {
		t.Fatal("panicking check returned nil")
	}
	if state.Value != 1 {
		t.Fatalf("panicking check retained tentative value: %d", state.Value)
	}
	if !mu.TryLock() {
		t.Fatal("panicking check retained store lock")
	}
	mu.Unlock()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-tr.OutCh:
			if msg.What == what.Patch {
				if msg.Data != `={"value":1}` {
					t.Fatalf("panic correction = %q", msg.Data)
				}
				return
			}
		case <-timer.C:
			t.Fatal("timed out waiting for panic correction")
		}
	}
}

func TestJsVarStoreRejectedUnhandledCheckIsHandled(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Value int `json:"value"`
	}{Value: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	rejection := fmt.Errorf("policy: %w", jaws.ErrEventUnhandled)
	store.ClientCheck = func(*jaws.Element, *struct {
		Value int `json:"value"`
	}, string,
	) error {
		return rejection
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	err := jaws.CallEventHandlers(binding, elem, what.Proposal, "value=2")
	if err == nil || errors.Is(err, jaws.ErrEventUnhandled) || !errors.Is(err, rejection) {
		t.Fatalf("rejected proposal dispatch = %v", err)
	}
	if state.Value != 1 {
		t.Fatalf("rejected proposal retained tentative value: %d", state.Value)
	}
}

func TestJsVarStorePathsAndDeletion(t *testing.T) {
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := map[string]map[string]int{"players": {"alice": 1, "bob": 2}}
	store := newTestJsVarStore(t, jw, "players", &mu, &state)
	if changed, err := store.SetPath("players.alice", 3); err != nil || !changed {
		t.Fatalf("SetPath = (%t, %v)", changed, err)
	}
	if changed, err := store.DeletePath("players.bob"); err != nil || !changed {
		t.Fatalf("DeletePath = (%t, %v)", changed, err)
	}
	if _, ok := state["players"]["bob"]; ok {
		t.Fatal("DeletePath left the map entry")
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.projectPatch("players.alice", encoded); got != "players.alice=3" {
		t.Fatalf("nested map patch = %q", got)
	}
	if got := store.projectPatch("players.bob", encoded); got != "players.bob=" {
		t.Fatalf("deletion patch = %q", got)
	}
	if changed, err := store.DeletePath("players.bob"); err != nil || changed {
		t.Fatalf("missing deletion = (%t, %v)", changed, err)
	}
	for _, path := range []string{".", "players..alice", "players.__proto__", "players.constructor", "players.prototype", "players=alice", "players\tbad", "players\x1bbad", "players\x7fbad"} {
		if changed, err := store.SetPath(path, 4); changed || !errors.Is(err, ErrIllegalJsVarPath) {
			t.Fatalf("invalid %q = (%t, %v)", path, changed, err)
		}
	}
	if !reflect.DeepEqual(state, map[string]map[string]int{"players": {"alice": 3}}) {
		t.Fatalf("invalid paths changed state: %#v", state)
	}
}

type jsVarOrdinaryUpdateProbe struct{ updates chan struct{} }

func (*jsVarOrdinaryUpdateProbe) JawsRender(*jaws.Element, io.Writer, []any) error { return nil }

func (probe *jsVarOrdinaryUpdateProbe) JawsUpdate(*jaws.Element) {
	select {
	case probe.updates <- struct{}{}:
	default:
	}
}

type jsVarLateExtraTagProbe struct {
	store *JsVarStore[struct {
		Count int `json:"count"`
	}]
	tag     any
	ready   chan struct{}
	release chan struct{}
	updates chan int
}

func (probe *jsVarLateExtraTagProbe) JawsRender(elem *jaws.Element, w io.Writer, _ []any) error {
	var count int
	probe.store.ReadLocked(func(value *struct {
		Count int `json:"count"`
	},
	) {
		count = value.Count
	})
	close(probe.ready)
	<-probe.release
	elem.Tag(probe.tag)
	_, err := io.WriteString(w, strconv.Itoa(count))
	return err
}

func (probe *jsVarLateExtraTagProbe) JawsUpdate(*jaws.Element) {
	probe.store.ReadLocked(func(value *struct {
		Count int `json:"count"`
	},
	) {
		probe.updates <- value.Count
	})
}

func TestJsVarStoreExtraTagsUpdateOrdinarySubscriber(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	t.Cleanup(jw.Close)
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	var mu sync.RWMutex
	value := struct {
		Count int `json:"count"`
	}{Count: 1}
	store := newTestJsVarStore(t, jw, "counter", &mu, &value)
	selector := tag.Tag("aggregate")
	store.ExtraTags = []any{selector}
	probe := &jsVarOrdinaryUpdateProbe{updates: make(chan struct{}, 1)}
	elem := tr.Request.NewElement(probe)
	elem.Tag(selector)
	if changed, err := store.SetPath("count", 2); err != nil || !changed {
		t.Fatalf("SetPath = (%t, %v)", changed, err)
	}
	select {
	case <-probe.updates:
	case <-time.After(5 * time.Second):
		t.Fatal("ExtraTags did not update ordinary subscriber")
	}
}

func TestJsVarStoreExtraTagsDuringPendingRender(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	t.Cleanup(jw.Close)
	active := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		active.Close()
		<-active.DoneCh
	})
	<-active.ReadyCh

	var mu sync.RWMutex
	value := struct {
		Count int `json:"count"`
	}{Count: 1}
	store := newTestJsVarStore(t, jw, "counter", &mu, &value)
	selector := tag.Tag("aggregate")
	store.ExtraTags = []any{selector}
	activeProbe := &jsVarOrdinaryUpdateProbe{updates: make(chan struct{}, 1)}
	active.Request.NewElement(activeProbe).Tag(selector)

	initial := httptest.NewRequest(http.MethodGet, "/", nil)
	pending := jw.NewRequest(httptest.NewRecorder(), initial)
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	probe := &jsVarLateExtraTagProbe{
		store:   store,
		tag:     selector,
		ready:   make(chan struct{}),
		release: release,
		updates: make(chan int, 1),
	}
	elem := pending.NewElement(probe)
	var html bytes.Buffer
	rendered := make(chan error, 1)
	go func() { rendered <- elem.JawsRender(&html, nil) }()
	select {
	case <-probe.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("pending render did not take its snapshot")
	}
	if changed, err := store.SetPath("count", 2); err != nil || !changed {
		t.Fatalf("SetPath = (%t, %v)", changed, err)
	}
	// Active subscriber proves a real update pass happened before pending render
	// registers the same tag.
	select {
	case <-activeProbe.updates:
	case <-time.After(5 * time.Second):
		t.Fatal("active subscriber did not receive update")
	}
	close(release)
	if err := <-rendered; err != nil {
		t.Fatal(err)
	}
	if got := html.String(); got != "1" {
		t.Fatalf("initial HTML = %q, want stale snapshot 1", got)
	}
	if claimed := jw.UseRequest(pending.JawsKey, initial); claimed != pending {
		t.Fatal("could not claim pending Request")
	}
	in, _, _, ready, done := jw.TestServe(pending, func(any) {})
	defer func() {
		close(in)
		<-done
	}()
	<-ready
	select {
	case got := <-probe.updates:
		if got != 2 {
			t.Fatalf("pending update = %d, want 2", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending Request missed ExtraTags update")
	}
}

func TestJsVarStoreWriteLocked(t *testing.T) {
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		X int `json:"x"`
		Y int `json:"y"`
	}{X: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	stop := errors.New("stop")
	err := store.WriteLocked(func(get func(string) (any, error), set func(string, any) (bool, error), _ func(string) (bool, error)) error {
		value, err := get("x")
		if err != nil {
			return err
		}
		if _, err = set("x", value.(int)+1); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) || state.X != 2 {
		t.Fatalf("earlier grouped write lost: state=%+v err=%v", state, err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("missing callback panic")
			}
		}()
		_ = store.WriteLocked(func(_ func(string) (any, error), set func(string, any) (bool, error), _ func(string) (bool, error)) error {
			if _, err := set("y", 4); err != nil {
				t.Fatal(err)
			}
			panic("stop")
		})
	}()
	if state.Y != 4 {
		t.Fatalf("panicking grouped write lost: %+v", state)
	}
	if changed, err := store.SetPath("x", 5); err != nil || !changed {
		t.Fatalf("lock leaked after panic: (%t, %v)", changed, err)
	}
}

type customJsVarState struct{ Value int }

func (value customJsVarState) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]int{"value": value.Value, "double": 2 * value.Value})
}

type panicMarshalJsVarState struct {
	Panic bool
	Value int
}

func (value panicMarshalJsVarState) MarshalJSON() ([]byte, error) {
	if value.Panic {
		panic("marshal")
	}
	return json.Marshal(struct {
		Value int `json:"value"`
	}{Value: value.Value})
}

func TestJsVarStoreProjectionFallback(t *testing.T) {
	if plainJsVarType(reflect.TypeFor[customJsVarState]()) {
		t.Fatal("custom marshaler considered independently patchable")
	}
	custom := JsVarStore[customJsVarState]{partial: plainJsVarType(reflect.TypeFor[customJsVarState]())}
	if got := custom.projectPatch("value", []byte(`{"value":2,"double":4}`)); got != `={"value":2,"double":4}` {
		t.Fatalf("custom marshal patch = %q", got)
	}
	if plainJsVarType(reflect.TypeFor[[]byte]()) {
		t.Fatal("byte slice considered independently patchable")
	}
	omit := JsVarStore[struct {
		Value string `json:"value,omitempty"`
	}]{partial: true}
	if got := omit.projectPatch("value", []byte(`{}`)); got != "value=" {
		t.Fatalf("omitted field patch = %q", got)
	}
	quoted := JsVarStore[struct {
		Count int `json:"count,string"`
	}]{partial: true}
	if got := quoted.projectPatch("count", []byte(`{"count":"42"}`)); got != `count="42"` {
		t.Fatalf("string-tag patch = %q", got)
	}
}

func TestJsVarStoreOmittedParentUsesRootPatch(t *testing.T) {
	type inner struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		S inner `json:"s,omitzero"`
	}{}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	if store.partial {
		t.Fatal("omittable parent permits descendant patches")
	}
	binding := store.Bind()
	if changed, err := store.SetPath("s.y", 5); err != nil || !changed {
		t.Fatalf("create s: (%t, %v)", changed, err)
	}
	patches, err := binding.snapshotPatches([]string{"s.y"})
	if err != nil || !reflect.DeepEqual(patches, []string{`={"s":{"x":0,"y":5}}`}) {
		t.Fatalf("create patch = %q, %v", patches, err)
	}
	if changed, err := store.SetPath("s.y", 0); err != nil || !changed {
		t.Fatalf("omit s: (%t, %v)", changed, err)
	}
	patches, err = binding.snapshotPatches([]string{"s.y"})
	if err != nil || !reflect.DeepEqual(patches, []string{`={}`}) {
		t.Fatalf("omit patch = %q, %v", patches, err)
	}
}

func TestJsVarStoreOmitEmptyMapUsesRootPatch(t *testing.T) {
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		S map[string]int `json:"s,omitempty"`
	}{}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	if store.partial {
		t.Fatal("omittable map permits descendant patches")
	}
	if changed, err := store.SetPath("s", map[string]int{"x": 1}); err != nil || !changed {
		t.Fatalf("create s: (%t, %v)", changed, err)
	}
	patches, err := store.Bind().snapshotPatches([]string{"s.x"})
	if err != nil || !reflect.DeepEqual(patches, []string{`={"s":{"x":1}}`}) {
		t.Fatalf("create patch = %q, %v", patches, err)
	}
}

func TestJsVarStoreOmittedScalarRejectsProposal(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Count int `json:"count,omitempty"`
	}{}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	if store.partial {
		t.Fatal("omitted scalar permits unchecked browser path")
	}
	checks := 0
	store.ClientCheck = func(*jaws.Element, *struct {
		Count int `json:"count,omitempty"`
	}, string,
	) error {
		checks++
		return nil
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := binding.JawsInput(elem, "count=1"); !errors.Is(err, ErrIllegalJsVarPath) || state.Count != 0 || checks != 0 {
		t.Fatalf("omitted path proposal: state=%+v checks=%d err=%v", state, checks, err)
	}
	if changed, err := store.SetPath("count", 1); err != nil || !changed {
		t.Fatalf("server create count: (%t, %v)", changed, err)
	}
	patches, err := binding.snapshotPatches([]string{"count"})
	if err != nil || !reflect.DeepEqual(patches, []string{`={"count":1}`}) {
		t.Fatalf("server create patch = %q, %v", patches, err)
	}
}

func TestJsVarStoreDeduplicatesWidenedPatches(t *testing.T) {
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Grid [70]int `json:"grid"`
	}{}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	if !store.partial {
		t.Fatal("fixed array unexpectedly uses root patches")
	}
	paths := make([]string, 64)
	for i := range paths {
		paths[i] = "grid." + strconv.Itoa(i)
	}
	patches, err := store.Bind().snapshotPatches(paths)
	encoded, encodeErr := json.Marshal(state.Grid)
	if err != nil || encodeErr != nil || !reflect.DeepEqual(patches, []string{"grid=" + string(encoded)}) {
		t.Fatalf("widened patches = %q, %v; encoding error = %v", patches, err, encodeErr)
	}
	patches, err = store.Bind().snapshotPatches([]string{"grid\nPatch\tother"})
	root, encodeErr := json.Marshal(state)
	if err != nil || encodeErr != nil || !reflect.DeepEqual(patches, []string{"=" + string(root)}) {
		t.Fatalf("invalid path fallback = %q, %v; encoding error = %v", patches, err, encodeErr)
	}
}

func TestJsVarStorePatchPreservesExactInteger(t *testing.T) {
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Value uint64 `json:"value"`
	}{Value: ^uint64(0)}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	patches, err := store.Bind().snapshotPatches([]string{"value"})
	if err != nil || !reflect.DeepEqual(patches, []string{"value=18446744073709551615"}) {
		t.Fatalf("integer patch = %q, %v", patches, err)
	}
}

func TestJsVarStoreRecursiveMapUsesRootPatch(t *testing.T) {
	type tree map[string]tree
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	state := tree{"leaf": {}}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	if store.partial {
		t.Fatal("recursive map permits descendant patches")
	}
}

func TestDecodeJsVarJSONRejectsDuplicateMembers(t *testing.T) {
	for _, data := range []string{
		`{"x":1,"x":2}`,
		`{"x":1,"\u0078":2}`,
		`{"outer":{"x":1,"x":2}}`,
		`[{"x":1,"x":2}]`,
	} {
		if _, err := decodeJsVarJSON([]byte(data)); err == nil {
			t.Errorf("accepted duplicate members in %s", data)
		}
	}
	if value, err := decodeJsVarJSON([]byte(`{"x":[1,2]}`)); err != nil || !reflect.DeepEqual(value, map[string]any{"x": []any{json.Number("1"), json.Number("2")}}) {
		t.Fatalf("valid JSON = %#v, %v", value, err)
	}
}

func TestJsVarStoreRejectsCollidingMapKeysOnRender(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := map[string]int{
		string([]byte{0xff, 0xfe}): 1,
		string([]byte{0x80, 0x81}): 2,
	}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	binding := store.Bind()
	elem := rq.NewElement(binding)
	if _, err := binding.renderSnapshot(elem); err == nil {
		t.Fatal("rendered colliding encoded map keys")
	}
}

func TestJsVarStoreRejectsEncoderAlias(t *testing.T) {
	type meta struct {
		Owner string `json:"owner"`
	}
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Meta meta `json:",embed"`
		Temp int  `json:"temp°C"`
	}{Meta: meta{Owner: "alice"}}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"Meta"`)) {
		t.Skip("encoder uses legacy field names")
	}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	if store.partial {
		t.Fatal("encoder aliases permit partial patches")
	}
	checks := 0
	store.ClientCheck = func(*jaws.Element, *struct {
		Meta meta `json:",embed"`
		Temp int  `json:"temp°C"`
	}, string,
	) error {
		checks++
		return nil
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	for _, proposal := range []string{`Meta={"owner":"mallory"}`, `Meta.owner="mallory"`, `Temp=99`} {
		if err := binding.JawsInput(elem, proposal); err == nil {
			t.Errorf("accepted encoder alias %q", proposal)
		}
	}
	if checks != 0 || state.Meta.Owner != "alice" || state.Temp != 0 {
		t.Fatalf("alias changed state: %+v, checks=%d", state, checks)
	}
}

func TestJsVarStoreRejectsEncoderAliasCollision(t *testing.T) {
	type meta struct {
		Owner string `json:"owner"`
	}
	type stateType struct {
		Meta  meta              `json:",embed"`
		Extra map[string]string `json:",embed"`
	}
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := stateType{Meta: meta{Owner: "alice"}, Extra: map[string]string{"Meta": "decoy"}}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	visible, err := decodeJsVarJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	object, ok := visible.(map[string]any)
	if !ok || object["owner"] != "alice" || object["Meta"] != "decoy" {
		t.Skip("encoder does not flatten embedded fields")
	}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	store.ClientCheck = func(*jaws.Element, *stateType, string) error { return nil }
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := binding.JawsInput(elem, `Meta={"owner":"mallory"}`); err == nil {
		t.Fatal("accepted path to unrelated visible member")
	}
	if state.Meta.Owner != "alice" || state.Extra["Meta"] != "decoy" {
		t.Fatalf("alias collision changed state: %+v", state)
	}
}

func TestJsVarStoreVisibleArrayWrite(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Items []int `json:"items"`
	}{Items: []int{1, 2}}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	checks := 0
	store.ClientCheck = func(*jaws.Element, *struct {
		Items []int `json:"items"`
	}, string,
	) error {
		checks++
		return nil
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := binding.JawsInput(elem, "items.1=3"); err != nil || !reflect.DeepEqual(state.Items, []int{1, 3}) || checks != 1 {
		t.Fatalf("array item write: %+v, checks=%d, %v", state, checks, err)
	}
	if err := binding.JawsInput(elem, "items.2=4"); !errors.Is(err, jq.ErrPathNotFound) || !reflect.DeepEqual(state.Items, []int{1, 3}) {
		t.Fatalf("array append: %+v, %v", state, err)
	}
}

func TestJsVarStoreRejectsNonFiniteConversion(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := struct {
		Value float32 `json:"value"`
	}{Value: 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	store.ClientCheck = func(*jaws.Element, *struct {
		Value float32 `json:"value"`
	}, string,
	) error {
		return nil
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := binding.JawsInput(elem, "value=3.5e38"); err == nil || state.Value != 1 {
		t.Fatalf("non-finite conversion: %v, %v", state.Value, err)
	}
}

func TestJsVarStoreRejectsDuplicateEncoding(t *testing.T) {
	type stateType struct {
		Status string
		Extra  map[string]string `json:",embed"`
	}
	duplicate := stateType{Status: "pending", Extra: map[string]string{"Status": "approved"}}
	data, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeJsVarJSON(data); err == nil {
		t.Skip("encoder does not flatten embedded fallback map")
	}
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := stateType{Status: "pending", Extra: map[string]string{}}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	store.ClientCheck = func(*jaws.Element, *stateType, string) error { return nil }
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := binding.JawsInput(elem, `={"Status":"pending","Extra":{"Status":"approved"}}`); err == nil {
		t.Fatal("accepted duplicate encoding")
	}
	if state.Extra["Status"] != "" || state.Status != "pending" {
		t.Fatalf("duplicate proposal retained: %+v", state)
	}
	state = duplicate
	duplicateBinding := store.Bind()
	duplicateElem := rq.NewElement(duplicateBinding)
	if _, err := duplicateBinding.renderSnapshot(duplicateElem); err == nil {
		t.Fatal("rendered duplicate JSON")
	}
	if _, err := binding.snapshotPatches([]string{""}); err == nil {
		t.Fatal("patched duplicate JSON")
	}
}

func TestJsVarStoreMarshalPanicReleasesLock(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	state := panicMarshalJsVarState{Panic: true}
	store := newTestJsVarStore(t, jw, "client", &mu, &state)
	binding := store.Bind()
	elem := rq.NewElement(binding)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("render did not panic")
			}
		}()
		_ = elem.JawsRender(&bytes.Buffer{}, nil)
	}()
	if !mu.TryLock() {
		t.Fatal("render panic retained store lock")
	}
	state.Panic = false
	mu.Unlock()
	rq.DeleteElement(elem)

	binding, elem, _ = renderTestJsVar(t, rq, store)
	if changed, err := store.SetPath("Panic", true); err != nil || !changed {
		t.Fatalf("SetPath = (%t, %v)", changed, err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("patch did not panic")
			}
		}()
		binding.JawsUpdatePaths(elem, []string{"Value"})
	}()
	if !mu.TryLock() {
		t.Fatal("patch panic retained store lock")
	}
	mu.Unlock()
}

func TestJsVarStoreNameConflictAndDeactivation(t *testing.T) {
	jw, rq := newCoreRequest(t)
	var mu sync.RWMutex
	value := 1
	first := newTestJsVarStore(t, jw, "client", &mu, &value)
	wrapped := &struct{ *JsVarBinding[int] }{first.Bind()}
	wrappedElem := rq.NewElement(wrapped)
	if err := wrappedElem.JawsRender(&bytes.Buffer{}, nil); err != nil {
		t.Fatalf("wrapped same-store binding: %v", err)
	}
	binding1, _, _ := renderTestJsVar(t, rq, first)
	if changed, err := first.SetPath("", 2); err != nil || !changed {
		t.Fatalf("SetPath = (%t, %v), want changed", changed, err)
	}
	binding2, _, html := renderTestJsVar(t, rq, first)
	if !strings.Contains(html, `data-jawsdata="2"`) {
		t.Fatalf("new binding snapshot = %q, want current value", html)
	}
	if !binding1.active.Load() || !binding2.active.Load() {
		t.Fatal("same-store bindings did not stay active")
	}
	other := newTestJsVarStore(t, jw, "client", &mu, &value)
	elem := rq.NewElement(other.Bind())
	if err := elem.JawsRender(&bytes.Buffer{}, nil); !errors.Is(err, ErrJsVarNameConflict) {
		t.Fatalf("different-store conflict: %v", err)
	}
	rq.DeleteElement(elem)
	binding2.Deactivate()
	if !binding1.active.Load() || binding2.active.Load() {
		t.Fatal("Deactivate affected another binding")
	}
	for _, name := range []string{"__proto__", "constructor", "prototype", strings.Repeat("a", 4097)} {
		if _, err := NewJsVarStore(jw, name, &mu, &value); !errors.Is(err, ErrIllegalJsVarName) {
			t.Fatalf("illegal name %q: %v", name, err)
		}
	}
	if _, err := NewJsVarStore(jw, strings.Repeat("a", 4096), &mu, &value); err != nil {
		t.Fatalf("maximum name length: %v", err)
	}
}

func TestJsVarStoreDeleteRejectsNamedStringMapKey(t *testing.T) {
	type key string
	jw, _ := newCoreRequest(t)
	var mu sync.RWMutex
	value := map[key]int{"a": 1}
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	if changed, err := store.DeletePath("a"); changed || !errors.Is(err, jq.ErrPathNotFound) {
		t.Fatalf("named key deletion = (%t, %v)", changed, err)
	}
}
