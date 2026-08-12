package ui

import (
	"context"
	"errors"
	"html/template"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type valueTestChild int

func (valueTestChild) JawsRender(elem *jaws.Element, w io.Writer, _ []any) (err error) {
	b := elem.Jid().AppendStartTagAttr(nil, "span")
	b = append(b, "></span>"...)
	_, err = w.Write(b)
	return
}

func (valueTestChild) JawsUpdate(*jaws.Element) {}

type valueTestProvider struct {
	mu       sync.Mutex
	children []jaws.UI
	selected string
	tag      any

	containsCalls int
	getCalls      int
	setCalls      int
	tagCalls      int
	attrCalls     int
}

func (p *valueTestProvider) JawsContains(*jaws.Element) []jaws.UI {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.containsCalls++
	return p.children
}

func (p *valueTestProvider) JawsGet(*jaws.Element) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.getCalls++
	return p.selected
}

func (p *valueTestProvider) JawsSet(_ *jaws.Element, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.setCalls++
	p.selected = value
	return nil
}

func (p *valueTestProvider) JawsGetTag() any {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tagCalls++
	return p.tag
}

func (p *valueTestProvider) JawsInitialHTMLAttr(*jaws.Element) template.HTMLAttr {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attrCalls++
	return `data-probe="true"`
}

func (p *valueTestProvider) callbackCount() int {
	snapshot := p.snapshot()
	return snapshot.containsCalls + snapshot.getCalls + snapshot.setCalls + snapshot.tagCalls + snapshot.attrCalls
}

type valueTestProviderSnapshot struct {
	selected      string
	containsCalls int
	getCalls      int
	setCalls      int
	tagCalls      int
	attrCalls     int
}

func (p *valueTestProvider) snapshot() (snapshot valueTestProviderSnapshot) {
	p.mu.Lock()
	snapshot = valueTestProviderSnapshot{
		selected:      p.selected,
		containsCalls: p.containsCalls,
		getCalls:      p.getCalls,
		setCalls:      p.setCalls,
		tagCalls:      p.tagCalls,
		attrCalls:     p.attrCalls,
	}
	p.mu.Unlock()
	return
}

type valueTestWidgetCase struct {
	name                string
	build               func(*valueTestProvider) jaws.UI
	wantLazyGetterCalls int
}

func valueTestWidgetCases() []valueTestWidgetCase {
	return []valueTestWidgetCase{
		{
			name:  "Container",
			build: func(p *valueTestProvider) jaws.UI { return NewContainer("div", p) },
		},
		{
			name:  "Tbody",
			build: func(p *valueTestProvider) jaws.UI { return NewTbody(p) },
		},
		{
			name:                "Select",
			build:               func(p *valueTestProvider) jaws.UI { return NewSelect(p) },
			wantLazyGetterCalls: 1,
		},
	}
}

type valueTestStateSnapshot struct {
	state     *containerState
	rendering bool
	dirtyTag  any
	children  []*jaws.Element
}

func valueTestSnapshotState(t *testing.T, elem *jaws.Element) (snapshot valueTestStateSnapshot) {
	t.Helper()
	state, ok := jaws.ElementState(elem).(*containerState)
	if !ok || state == nil {
		t.Fatalf("Element state = %T, want non-nil *containerState", jaws.ElementState(elem))
	}
	state.mu.Lock()
	snapshot = valueTestStateSnapshot{
		state:     state,
		rendering: state.rendering,
		dirtyTag:  state.dirtyTag,
		children:  append([]*jaws.Element(nil), state.contents...),
	}
	state.mu.Unlock()
	return
}

type valueTestRebuildingProvider struct {
	build func() jaws.UI
}

func (p *valueTestRebuildingProvider) JawsContains(*jaws.Element) []jaws.UI {
	return []jaws.UI{p.build()}
}

type valueTestIncomparableProvider []jaws.UI

func (p valueTestIncomparableProvider) JawsContains(*jaws.Element) []jaws.UI {
	return p
}

type valueTestWriteCounter struct{ calls int }

func (w *valueTestWriteCounter) Write(p []byte) (int, error) {
	w.calls++
	return len(p), nil
}

type valueTestLogger struct {
	log testErrorLog
}

func (*valueTestLogger) Info(string, ...any) {}
func (*valueTestLogger) Warn(string, ...any) {}
func (l *valueTestLogger) Error(_ string, args ...any) {
	l.log.record(args)
}

func (l *valueTestLogger) sync(t *testing.T, jw *jaws.Jaws) []error {
	return l.log.sync(t, jw)
}

type valueTestBlockingWriter struct {
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
	releaseOnce sync.Once
	writes      atomic.Int32
}

func valueTestNewBlockingWriter() *valueTestBlockingWriter {
	return &valueTestBlockingWriter{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (w *valueTestBlockingWriter) Write(p []byte) (n int, err error) {
	w.writes.Add(1)
	block := false
	w.startOnce.Do(func() {
		block = true
		close(w.started)
	})
	if block {
		<-w.release
	}
	return len(p), nil
}

func (w *valueTestBlockingWriter) Release() {
	w.releaseOnce.Do(func() { close(w.release) })
}

type valueTestUpdateProbe struct {
	updates atomic.Int32
	called  chan struct{}
}

func (*valueTestUpdateProbe) JawsRender(*jaws.Element, io.Writer, []any) error { return nil }
func (p *valueTestUpdateProbe) JawsUpdate(*jaws.Element) {
	p.updates.Add(1)
	if p.called != nil {
		select {
		case p.called <- struct{}{}:
		default:
		}
	}
}

func valueTestNewLoopRequest(t *testing.T, logger jaws.Logger) *jawstest.TestRequest {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	jw.Logger = logger
	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh
	return tr
}

func valueTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(t.Context(), 5*time.Second)
}

func valueTestAssertNoQueuedElementWork(t *testing.T, tr *jawstest.TestRequest) {
	t.Helper()
	const probe = "value-state-no-update-work"
	ctx, cancel := valueTestContext(t)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("timed out waking the request loop")
	case tr.InCh <- wire.WsMsg{}:
	}
	select {
	case <-ctx.Done():
		t.Fatal("timed out sending the output probe")
	case tr.BcastCh <- wire.Message{What: what.Alert, Data: probe}:
	}
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timed out draining update output")
		case msg, ok := <-tr.OutCh:
			if !ok {
				t.Fatal("request loop stopped before the output probe arrived")
			}
			if msg.What == what.Alert && msg.Data == probe {
				return
			}
			if msg.Jid != 0 {
				t.Fatalf("state contention queued browser work %v for %v", msg.What, msg.Jid)
			}
		}
	}
}

func valueTestWaitForProbe(t *testing.T, probe <-chan struct{}) {
	t.Helper()
	ctx, cancel := valueTestContext(t)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("timed out waiting for update probe")
	case <-probe:
	}
}

func valueTestRequirePanic(t *testing.T, fn func() error) {
	t.Helper()
	var callErr error
	defer func() {
		if recover() == nil {
			t.Fatalf("call returned %v, want panic", callErr)
		}
	}()
	callErr = fn()
}

func TestContainerValueConstructorsAndMethodSets(t *testing.T) {
	provider := &valueTestProvider{}
	container := NewContainer("div", provider)
	tbody := NewTbody(provider)
	selectUI := NewSelect(provider)
	if want := NewContainer("select", provider); selectUI.container() != want {
		t.Fatalf("Select container definition = %#v, want %#v", selectUI.container(), want)
	}
	if zero, nilHandler := (Select{}), NewSelect(nil); zero != nilHandler {
		t.Fatalf("zero Select = %#v, want equality with NewSelect(nil) %#v", zero, nilHandler)
	}

	// Concrete assignments pin the constructor result types; the interface
	// assignments pin the method sets of those values rather than pointer wrappers.
	var (
		containerValue Container         = container
		tbodyValue     Tbody             = tbody
		selectValue    Select            = selectUI
		containerUI    jaws.UI           = containerValue
		tbodyUI        jaws.UI           = tbodyValue
		selectWidget   jaws.UI           = selectValue
		selectInput    jaws.InputHandler = selectValue
	)

	for _, tt := range []struct {
		name  string
		value any
	}{
		{"Container", containerUI},
		{"Tbody", tbodyUI},
		{"Select UI", selectWidget},
		{"Select input", selectInput},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := reflect.TypeOf(tt.value).Kind(); got != reflect.Struct {
				t.Fatalf("constructor dynamic kind = %v, want struct value", got)
			}
		})
	}
}

// TestContainerValueRebuildReusesElement covers the value widgets as children of a
// dynamic container. Each provider call constructs a fresh but equal child value.
func TestContainerValueRebuildReusesElement(t *testing.T) {
	for _, tt := range valueTestWidgetCases() {
		t.Run(tt.name, func(t *testing.T) {
			tr := newReuseRequest(t)
			definition := &valueTestProvider{
				children: []jaws.UI{valueTestChild(1)},
				selected: "one",
			}
			rebuilding := &valueTestRebuildingProvider{
				build: func() jaws.UI { return tt.build(definition) },
			}
			outer := NewContainer("div", rebuilding)
			elem := tr.NewElement(outer)
			if err := elem.JawsRender(io.Discard, nil); err != nil {
				t.Fatal(err)
			}

			before := valueTestSnapshotState(t, elem)
			if len(before.children) != 1 {
				t.Fatalf("children after render = %d, want 1", len(before.children))
			}
			beforeChild := before.children[0]

			outer.JawsUpdate(elem)

			after := valueTestSnapshotState(t, elem)
			if len(after.children) != 1 {
				t.Fatalf("children after update = %d, want 1", len(after.children))
			}
			if got := after.children[0].Jid(); got != beforeChild.Jid() {
				t.Fatalf("rebuilt child Jid = %v, want retained Jid %v", got, beforeChild.Jid())
			}
			if after.children[0] != beforeChild {
				t.Fatal("rebuilt equal value did not retain its child Element")
			}
			assertNoDOMMutation(t, tr, 1)
		})
	}
}

func TestEqualContainerValuesKeepIndependentElementState(t *testing.T) {
	for _, tt := range valueTestWidgetCases() {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			provider := &valueTestProvider{
				children: []jaws.UI{valueTestChild(1)},
				selected: "one",
			}
			first := rq.NewElement(tt.build(provider))
			second := rq.NewElement(tt.build(provider))
			if first.UI() != second.UI() {
				t.Fatal("rebuilt widget values are not equal")
			}
			if err := first.JawsRender(io.Discard, nil); err != nil {
				t.Fatal(err)
			}
			if err := second.JawsRender(io.Discard, nil); err != nil {
				t.Fatal(err)
			}

			firstState := valueTestSnapshotState(t, first)
			secondState := valueTestSnapshotState(t, second)
			if firstState.state == secondState.state {
				t.Fatal("equal values share *containerState")
			}
			if len(firstState.children) != 1 || len(secondState.children) != 1 {
				t.Fatalf("child counts = %d and %d, want 1 each", len(firstState.children), len(secondState.children))
			}
			if firstState.children[0] == secondState.children[0] {
				t.Fatal("equal values share a child Element")
			}
			if firstState.children[0].Jid() == secondState.children[0].Jid() {
				t.Fatalf("independent child Elements share Jid %v", firstState.children[0].Jid())
			}
		})
	}
}

func TestContainerRenderClaimsStateBeforeSideEffects(t *testing.T) {
	for _, tt := range valueTestWidgetCases() {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			getterTag := new(int)
			paramTag := new(int)
			provider := &valueTestProvider{
				children: []jaws.UI{valueTestChild(1)},
				selected: "one",
				tag:      getterTag,
			}
			widget := tt.build(provider)
			elem := rq.NewElement(widget)
			if err := jaws.SetElementState(elem, new(int)); err != nil {
				t.Fatal(err)
			}
			writer := new(valueTestWriteCounter)

			err := widget.JawsRender(elem, writer, []any{paramTag})
			if !errors.Is(err, jaws.ErrElementStateClaimed) {
				t.Fatalf("render = %v, want %v", err, jaws.ErrElementStateClaimed)
			}
			if got := provider.callbackCount(); got != 0 {
				t.Fatalf("provider/getter callbacks before rejected claim = %d, want 0", got)
			}
			if writer.calls != 0 {
				t.Fatalf("writes before rejected claim = %d, want 0", writer.calls)
			}
			if got := len(rq.GetElements(getterTag)); got != 0 {
				t.Fatalf("Elements registered under getter tag = %d, want 0", got)
			}
			if got := len(rq.GetElements(paramTag)); got != 0 {
				t.Fatalf("Elements registered under param tag = %d, want 0", got)
			}
		})
	}
}

func TestContainerUpdateRejectsRenderInProgress(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func(*valueTestProvider) jaws.UI
	}{
		{"Container", func(p *valueTestProvider) jaws.UI { return NewContainer("div", p) }},
		{"Tbody", func(p *valueTestProvider) jaws.UI { return NewTbody(p) }},
		{"Select", func(p *valueTestProvider) jaws.UI { return NewSelect(p) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := new(valueTestLogger)
			tr := valueTestNewLoopRequest(t, logger)
			provider := &valueTestProvider{
				children: []jaws.UI{valueTestChild(1)},
				selected: "one",
				tag:      new(int),
			}
			widget := tt.build(provider)
			elem := tr.NewElement(widget)
			writer := valueTestNewBlockingWriter()
			defer writer.Release()
			renderDone := make(chan error, 1)
			go func() { renderDone <- elem.JawsRender(writer, nil) }()

			ctx, cancel := valueTestContext(t)
			defer cancel()
			select {
			case <-ctx.Done():
				t.Fatal("render did not reach its first wrapper write")
			case <-writer.started:
			}
			if snapshot := valueTestSnapshotState(t, elem); !snapshot.rendering {
				t.Fatal("state was published as complete while the render was blocked")
			}

			elem.JawsUpdate()

			logged := logger.sync(t, tr.Jaws)
			if len(logged) != 1 || !errors.Is(logged[0], jaws.ErrElementStateClaimed) {
				t.Fatalf("logged errors = %v, want one %v", logged, jaws.ErrElementStateClaimed)
			}
			calls := provider.snapshot()
			if calls.containsCalls != 0 || calls.getCalls != 0 {
				t.Fatalf("callbacks during render contention = contains:%d get:%d, want 0", calls.containsCalls, calls.getCalls)
			}
			if got := writer.writes.Load(); got != 1 {
				t.Fatalf("render writes while blocked = %d, want 1", got)
			}
			valueTestAssertNoQueuedElementWork(t, tr)

			writer.Release()
			select {
			case <-ctx.Done():
				t.Fatal("render did not finish after release")
			case err := <-renderDone:
				if err != nil {
					t.Fatal(err)
				}
			}
			finished := valueTestSnapshotState(t, elem)
			if finished.rendering {
				t.Fatal("completed render left the state in rendering phase")
			}
			if len(finished.children) != 1 {
				t.Fatalf("completed render children = %d, want 1", len(finished.children))
			}
		})
	}
}

func TestContainerUpdateRejectsClaimedStateBeforeProviderCallback(t *testing.T) {
	stateCases := []struct {
		name  string
		claim func(*jaws.Element) error
	}{
		{
			name:  "foreign",
			claim: func(elem *jaws.Element) error { return jaws.SetElementState(elem, new(int)) },
		},
		{
			name: "typed nil",
			claim: func(elem *jaws.Element) error {
				var state *containerState
				return jaws.SetElementState(elem, state)
			},
		},
	}

	for _, stateCase := range stateCases {
		t.Run(stateCase.name, func(t *testing.T) {
			for _, widgetCase := range valueTestWidgetCases() {
				t.Run(widgetCase.name, func(t *testing.T) {
					logger := new(valueTestLogger)
					jw, rq := newConfiguredCoreRequest(t, func(jw *jaws.Jaws) { jw.Logger = logger })
					provider := &valueTestProvider{
						children: []jaws.UI{valueTestChild(1)},
						selected: "one",
					}
					elem := rq.NewElement(widgetCase.build(provider))
					if err := stateCase.claim(elem); err != nil {
						t.Fatal(err)
					}

					elem.JawsUpdate()

					if got := provider.callbackCount(); got != 0 {
						t.Fatalf("provider callbacks = %d, want 0", got)
					}
					logged := logger.sync(t, jw)
					if len(logged) != 1 || !errors.Is(logged[0], jaws.ErrElementStateClaimed) {
						t.Fatalf("logged errors = %v, want one %v", logged, jaws.ErrElementStateClaimed)
					}
				})
			}
		})
	}
}

func TestClaimContainerStateLosesOccupiedSlot(t *testing.T) {
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(valueTestChild(1))
	winner := new(containerState)
	if err := jaws.SetElementState(elem, winner); err != nil {
		t.Fatal(err)
	}

	// The preclaim models another updater winning after this updater observed an
	// unclaimed slot but before its claim attempt acquired the Request lock.
	state, err := claimContainerState(elem)
	if !errors.Is(err, jaws.ErrElementStateClaimed) {
		t.Fatalf("claim = %v, want %v", err, jaws.ErrElementStateClaimed)
	}
	if state != nil {
		t.Fatalf("losing claim returned state %p, want nil", state)
	}
	if got := jaws.ElementState(elem); got != winner {
		t.Fatalf("losing claim replaced winner with %T", got)
	}
}

func TestContainerUpdateLazilyClaimsState(t *testing.T) {
	for _, tt := range valueTestWidgetCases() {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			provider := &valueTestProvider{
				children: []jaws.UI{valueTestChild(1)},
				selected: "one",
			}
			widget := tt.build(provider)
			jid := (RequestWriter{Request: rq}).Register(widget)
			elem := rq.GetElementByJid(jid)
			if elem == nil {
				t.Fatalf("Register Element %v is not registered", jid)
			}

			snapshot := valueTestSnapshotState(t, elem)
			if snapshot.rendering {
				t.Fatal("lazily claimed state remains in rendering phase")
			}
			if snapshot.dirtyTag != nil {
				t.Fatalf("lazy state's dirty tag = %v, want nil", snapshot.dirtyTag)
			}
			if len(snapshot.children) != 1 {
				t.Fatalf("lazy state's children = %d, want 1", len(snapshot.children))
			}
			calls := provider.snapshot()
			if calls.containsCalls != 1 {
				t.Fatalf("JawsContains calls = %d, want 1", calls.containsCalls)
			}
			if calls.getCalls != tt.wantLazyGetterCalls {
				t.Fatalf("JawsGet calls = %d, want %d", calls.getCalls, tt.wantLazyGetterCalls)
			}
			if calls.tagCalls != 0 || calls.attrCalls != 0 {
				t.Fatalf("render-only getter callbacks = tag:%d attr:%d, want 0", calls.tagCalls, calls.attrCalls)
			}
		})
	}
}

func TestSelectInputUsesSavedContainerTag(t *testing.T) {
	tr := valueTestNewLoopRequest(t, nil)
	savedTag := new(int)
	provider := &valueTestProvider{
		children: []jaws.UI{valueTestChild(1)},
		selected: "before",
		tag:      savedTag,
	}
	selectUI := NewSelect(provider)
	elem := tr.NewElement(selectUI)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	snapshot := valueTestSnapshotState(t, elem)
	if snapshot.dirtyTag != savedTag {
		t.Fatalf("saved dirty tag = %v, want exact getter tag %v", snapshot.dirtyTag, savedTag)
	}
	if !elem.HasTag(savedTag) {
		t.Fatal("render did not register the saved getter tag")
	}

	// Drain the selected value queued by initial rendering. The resulting update,
	// together with the unchanged tag-call count, proves input dirties the retained
	// tag instead of asking the provider to resolve it again.
	assertNoDOMMutation(t, tr, 1)
	before := provider.snapshot()
	if err := selectUI.JawsInput(elem, "after"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := valueTestContext(t)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("Select input did not dirty the Element through its saved tag")
		case msg, ok := <-tr.OutCh:
			if !ok {
				t.Fatal("request loop stopped before the Select value update arrived")
			}
			if msg.Jid == elem.Jid() && msg.What == what.Value && msg.Data == "after" {
				after := provider.snapshot()
				if after.setCalls != before.setCalls+1 {
					t.Fatalf("JawsSet calls = %d, want %d", after.setCalls, before.setCalls+1)
				}
				if after.tagCalls != before.tagCalls {
					t.Fatalf("input/update re-read getter tag %d times", after.tagCalls-before.tagCalls)
				}
				return
			}
		}
	}
}

func TestSelectInputWithoutUsableStateDoesNotClaim(t *testing.T) {
	stateCases := []struct {
		name  string
		claim func(*jaws.Element) error
	}{
		{name: "missing", claim: func(*jaws.Element) error { return nil }},
		{name: "foreign", claim: func(elem *jaws.Element) error {
			return jaws.SetElementState(elem, new(int))
		}},
		{name: "typed nil", claim: func(elem *jaws.Element) error {
			var state *containerState
			return jaws.SetElementState(elem, state)
		}},
	}

	for _, tt := range stateCases {
		t.Run(tt.name, func(t *testing.T) {
			tr := valueTestNewLoopRequest(t, nil)
			getterTag := new(int)
			provider := &valueTestProvider{selected: "before", tag: getterTag}
			selectUI := NewSelect(provider)
			elem := tr.NewElement(selectUI)
			if err := tt.claim(elem); err != nil {
				t.Fatal(err)
			}
			beforeState := jaws.ElementState(elem)
			beforeCalls := provider.snapshot()

			// If input supplies any render-derived tag instead of nil, this lower-Jid
			// observer runs before the barrier probe in the sorted update list.
			observer := new(valueTestUpdateProbe)
			observerElem := tr.NewElement(observer)
			observerElem.Tag(getterTag)
			observerElem.Freeze()
			barrierTag := new(int)
			barrier := &valueTestUpdateProbe{called: make(chan struct{}, 1)}
			barrierElem := tr.NewElement(barrier)
			barrierElem.Tag(barrierTag)
			barrierElem.Freeze()

			if err := selectUI.JawsInput(elem, "after"); err != nil {
				t.Fatal(err)
			}
			tr.Dirty(barrierTag)
			valueTestWaitForProbe(t, barrier.called)

			if got := jaws.ElementState(elem); got != beforeState {
				t.Fatalf("Element state changed from %T to %T", beforeState, got)
			}
			if got := observer.updates.Load(); got != 0 {
				t.Fatalf("getter-tag observer updates = %d, want 0 for a nil dirty tag", got)
			}
			afterCalls := provider.snapshot()
			if afterCalls.setCalls != beforeCalls.setCalls+1 || afterCalls.selected != "after" {
				t.Fatalf("handler after input = %+v, want one set to after", afterCalls)
			}
			if afterCalls.containsCalls != beforeCalls.containsCalls ||
				afterCalls.getCalls != beforeCalls.getCalls ||
				afterCalls.tagCalls != beforeCalls.tagCalls {
				t.Fatalf("input made non-setter callbacks: before=%+v after=%+v", beforeCalls, afterCalls)
			}
		})
	}
}

func TestContainerFamilyNilProviderPanics(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func() jaws.UI
	}{
		{"Container zero", func() jaws.UI { return Container{} }},
		{"Container nil provider", func() jaws.UI { return NewContainer("div", nil) }},
		{"Tbody zero", func() jaws.UI { return Tbody{} }},
		{"Tbody nil provider", func() jaws.UI { return NewTbody(nil) }},
		{"Select zero", func() jaws.UI { return Select{} }},
		{"Select nil handler", func() jaws.UI { return NewSelect(nil) }},
	} {
		for _, operation := range []struct {
			name string
			call func(*jaws.Element) error
		}{
			{"render", func(elem *jaws.Element) error { return elem.JawsRender(io.Discard, nil) }},
			{"update", func(elem *jaws.Element) error { elem.JawsUpdate(); return nil }},
		} {
			t.Run(tt.name+" "+operation.name, func(t *testing.T) {
				_, rq := newCoreRequest(t)
				elem := rq.NewElement(tt.build())
				valueTestRequirePanic(t, func() error { return operation.call(elem) })
			})
		}
	}
}

// TestSelectNilHandlerInputIsNoOp pins the input-side asymmetry: the same nil
// handler that panics on render and update is ignored by JawsInput.
func TestSelectNilHandlerInputIsNoOp(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func() Select
	}{
		{"zero", func() Select { return Select{} }},
		{"nil handler", func() Select { return NewSelect(nil) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			selectUI := tt.build()
			elem := rq.NewElement(selectUI)
			if err := selectUI.JawsInput(elem, "ignored"); err != nil {
				t.Fatal(err)
			}
			if state := jaws.ElementState(elem); state != nil {
				t.Fatalf("nil-handler Select input claimed state %T", state)
			}
		})
	}
}

func TestContainerDefinitionComparabilityBoundary(t *testing.T) {
	t.Run("direct incomparable value", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		rows := valueTestIncomparableProvider{valueTestChild(1)}
		outerProvider := &testContainer{contents: []jaws.UI{NewContainer("div", rows)}}
		elem := rq.NewElement(NewContainer("div", outerProvider))
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			t.Fatalf("render = %v, want cancellation instead", err)
		}
		if cause := context.Cause(rq.Context()); !errors.Is(cause, tag.ErrNotUsableAsTag) {
			t.Fatalf("cancellation cause = %v, want wrapping %v", cause, tag.ErrNotUsableAsTag)
		}
	})

	t.Run("stable pointer", func(t *testing.T) {
		tr := newReuseRequest(t)
		rows := valueTestIncomparableProvider{valueTestChild(1)}
		firstValue := NewContainer("div", &rows)
		if rebuilt := NewContainer("div", &rows); rebuilt != firstValue {
			t.Fatal("Containers rebuilt around the same provider pointer are not equal")
		}
		rebuilding := &valueTestRebuildingProvider{
			build: func() jaws.UI { return NewContainer("div", &rows) },
		}
		outer := NewContainer("div", rebuilding)
		elem := tr.NewElement(outer)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			t.Fatal(err)
		}
		before := valueTestSnapshotState(t, elem)
		if len(before.children) != 1 {
			t.Fatalf("children after render = %d, want 1", len(before.children))
		}

		elem.JawsUpdate()

		after := valueTestSnapshotState(t, elem)
		if len(after.children) != 1 || after.children[0] != before.children[0] {
			t.Fatal("stable provider pointer did not reuse the Container child Element")
		}
		assertNoDOMMutation(t, tr, 1)
	})
}
