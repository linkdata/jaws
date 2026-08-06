package ui

import (
	"cmp"
	"fmt"
	"html/template"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type nestedReorderTree struct {
	bodies []*nestedReorderBody
}

func (tree *nestedReorderTree) JawsContains(*jaws.Element) (contents []jaws.UI) {
	contents = make([]jaws.UI, len(tree.bodies))
	for i, body := range tree.bodies {
		contents[i] = NewTbody(body)
	}
	return
}

type nestedReorderBody struct {
	rows []*nestedReorderRow
}

func (body *nestedReorderBody) JawsContains(*jaws.Element) (contents []jaws.UI) {
	contents = make([]jaws.UI, len(body.rows))
	for i, row := range body.rows {
		contents[i] = NewContainer("tr", row)
	}
	return
}

type nestedReorderRow struct {
	cells []*nestedReorderCell
}

func (row *nestedReorderRow) JawsContains(*jaws.Element) (contents []jaws.UI) {
	contents = make([]jaws.UI, len(row.cells))
	for i, cell := range row.cells {
		contents[i] = NewContainer("td", cell)
	}
	return
}

type nestedReorderCell struct {
	selects []*nestedReorderSelect
}

func (cell *nestedReorderCell) JawsContains(*jaws.Element) (contents []jaws.UI) {
	contents = make([]jaws.UI, len(cell.selects))
	for i, selectHandler := range cell.selects {
		contents[i] = NewSelect(selectHandler)
	}
	return
}

type nestedReorderSelect struct {
	options  []*nestedReorderOption
	selected string
}

func (selectHandler *nestedReorderSelect) JawsContains(*jaws.Element) (contents []jaws.UI) {
	contents = make([]jaws.UI, len(selectHandler.options))
	for i, option := range selectHandler.options {
		contents[i] = nestedReorderOptionUI{option: option}
	}
	return
}

func (selectHandler *nestedReorderSelect) JawsGet(*jaws.Element) string {
	return selectHandler.selected
}

func (selectHandler *nestedReorderSelect) JawsSet(_ *jaws.Element, value string) (err error) {
	if selectHandler.selected == value {
		err = jaws.ErrValueUnchanged
	} else {
		selectHandler.selected = value
	}
	return
}

type nestedReorderOption struct {
	value       string
	label       string
	renderCount int
}

type nestedReorderOptionUI struct {
	option *nestedReorderOption
}

func (u nestedReorderOptionUI) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	u.option.renderCount++
	attrs := append(elem.ApplyParams(params), htmlio.Attr("value", u.option.value))
	err = htmlio.WriteHTMLInner(w, elem.Jid(), "option", "", template.HTML(template.HTMLEscapeString(u.option.label)), attrs...)
	return
}

func (nestedReorderOptionUI) JawsUpdate(*jaws.Element) {}

func newNestedReorderTree(bodyCount, rowCount, cellCount, selectCount, optionCount int) (tree *nestedReorderTree) {
	tree = &nestedReorderTree{bodies: make([]*nestedReorderBody, bodyCount)}
	for bodyIndex := range tree.bodies {
		body := &nestedReorderBody{rows: make([]*nestedReorderRow, rowCount)}
		tree.bodies[bodyIndex] = body
		for rowIndex := range body.rows {
			row := &nestedReorderRow{cells: make([]*nestedReorderCell, cellCount)}
			body.rows[rowIndex] = row
			for cellIndex := range row.cells {
				cell := &nestedReorderCell{selects: make([]*nestedReorderSelect, selectCount)}
				row.cells[cellIndex] = cell
				for selectIndex := range cell.selects {
					selectHandler := &nestedReorderSelect{options: make([]*nestedReorderOption, optionCount)}
					cell.selects[selectIndex] = selectHandler
					for optionIndex := range selectHandler.options {
						value := fmt.Sprintf("%d-%d-%d-%d-%d", bodyIndex, rowIndex, cellIndex, selectIndex, optionIndex)
						selectHandler.options[optionIndex] = &nestedReorderOption{
							value: value,
							label: "option " + value,
						}
					}
					if len(selectHandler.options) > 0 {
						selectHandler.selected = selectHandler.options[0].value
					}
				}
			}
		}
	}
	return
}

func (tree *nestedReorderTree) reverse() {
	slices.Reverse(tree.bodies)
	for _, body := range tree.bodies {
		slices.Reverse(body.rows)
		for _, row := range body.rows {
			slices.Reverse(row.cells)
			for _, cell := range row.cells {
				slices.Reverse(cell.selects)
				for _, selectHandler := range cell.selects {
					slices.Reverse(selectHandler.options)
				}
			}
		}
	}
}

type nestedReorderElementRef struct {
	elem *jaws.Element
	jid  jaws.Jid
}

type nestedReorderElements struct {
	root    nestedReorderElementRef
	bodies  map[*nestedReorderBody]nestedReorderElementRef
	rows    map[*nestedReorderRow]nestedReorderElementRef
	cells   map[*nestedReorderCell]nestedReorderElementRef
	selects map[*nestedReorderSelect]nestedReorderElementRef
	options map[*nestedReorderOption]nestedReorderElementRef
	updates []*jaws.Element
}

func newNestedReorderElements(tb testing.TB, root *jaws.Element, tree *nestedReorderTree) (elements *nestedReorderElements) {
	tb.Helper()
	elements = &nestedReorderElements{
		root:    nestedReorderElementRef{elem: root, jid: root.Jid()},
		bodies:  make(map[*nestedReorderBody]nestedReorderElementRef),
		rows:    make(map[*nestedReorderRow]nestedReorderElementRef),
		cells:   make(map[*nestedReorderCell]nestedReorderElementRef),
		selects: make(map[*nestedReorderSelect]nestedReorderElementRef),
		options: make(map[*nestedReorderOption]nestedReorderElementRef),
	}

	bodyElems := nestedReorderChildren(tb, root, len(tree.bodies))
	for bodyIndex, body := range tree.bodies {
		bodyElem := bodyElems[bodyIndex]
		nestedReorderRequireUI(tb, bodyElem, NewTbody(body))
		elements.bodies[body] = nestedReorderElementRef{elem: bodyElem, jid: bodyElem.Jid()}

		rowElems := nestedReorderChildren(tb, bodyElem, len(body.rows))
		for rowIndex, row := range body.rows {
			rowElem := rowElems[rowIndex]
			nestedReorderRequireUI(tb, rowElem, NewContainer("tr", row))
			elements.rows[row] = nestedReorderElementRef{elem: rowElem, jid: rowElem.Jid()}

			cellElems := nestedReorderChildren(tb, rowElem, len(row.cells))
			for cellIndex, cell := range row.cells {
				cellElem := cellElems[cellIndex]
				nestedReorderRequireUI(tb, cellElem, NewContainer("td", cell))
				elements.cells[cell] = nestedReorderElementRef{elem: cellElem, jid: cellElem.Jid()}

				selectElems := nestedReorderChildren(tb, cellElem, len(cell.selects))
				for selectIndex, selectHandler := range cell.selects {
					selectElem := selectElems[selectIndex]
					nestedReorderRequireUI(tb, selectElem, NewSelect(selectHandler))
					elements.selects[selectHandler] = nestedReorderElementRef{elem: selectElem, jid: selectElem.Jid()}

					optionElems := nestedReorderChildren(tb, selectElem, len(selectHandler.options))
					for optionIndex, option := range selectHandler.options {
						optionElem := optionElems[optionIndex]
						nestedReorderRequireUI(tb, optionElem, nestedReorderOptionUI{option: option})
						elements.options[option] = nestedReorderElementRef{elem: optionElem, jid: optionElem.Jid()}
					}
				}
			}
		}
	}
	elements.updates = make([]*jaws.Element, 0, 1+len(elements.bodies)+len(elements.rows)+len(elements.cells)+len(elements.selects))
	elements.updates = append(elements.updates, root)
	for _, ref := range elements.bodies {
		elements.updates = append(elements.updates, ref.elem)
	}
	for _, ref := range elements.rows {
		elements.updates = append(elements.updates, ref.elem)
	}
	for _, ref := range elements.cells {
		elements.updates = append(elements.updates, ref.elem)
	}
	for _, ref := range elements.selects {
		elements.updates = append(elements.updates, ref.elem)
	}
	slices.SortFunc(elements.updates, func(a, b *jaws.Element) int {
		return cmp.Compare(a.Jid(), b.Jid())
	})
	return
}

func nestedReorderChildren(tb testing.TB, elem *jaws.Element, want int) (children []*jaws.Element) {
	tb.Helper()
	state, ok := jaws.ElementState(elem).(*containerState)
	if !ok || state == nil {
		tb.Fatalf("Element %v state = %T, want *containerState", elem.Jid(), jaws.ElementState(elem))
	}
	state.mu.Lock()
	children = append(children, state.contents...)
	state.mu.Unlock()
	if len(children) != want {
		tb.Fatalf("Element %v has %d children, want %d", elem.Jid(), len(children), want)
	}
	return
}

func nestedReorderRequireUI(tb testing.TB, elem *jaws.Element, want jaws.UI) {
	tb.Helper()
	if got := elem.UI(); got != want {
		tb.Fatalf("Element %v UI = %#v, want %#v", elem.Jid(), got, want)
	}
}

func (ref nestedReorderElementRef) require(tb testing.TB, got *jaws.Element) {
	tb.Helper()
	if got != ref.elem {
		tb.Fatalf("Element = %v, want retained Element %v", got.Jid(), ref.jid)
	}
	if got.Jid() != ref.jid {
		tb.Fatalf("retained Element Jid = %v, want %v", got.Jid(), ref.jid)
	}
	if registered := got.Request.GetElementByJid(ref.jid); registered != ref.elem {
		tb.Fatalf("request Element %v = %p, want retained %p", ref.jid, registered, ref.elem)
	}
}

func (elements *nestedReorderElements) requireTree(tb testing.TB, tree *nestedReorderTree) {
	tb.Helper()
	bodyElems := nestedReorderChildren(tb, elements.root.elem, len(tree.bodies))
	for bodyIndex, body := range tree.bodies {
		bodyRef := elements.bodies[body]
		bodyRef.require(tb, bodyElems[bodyIndex])

		rowElems := nestedReorderChildren(tb, bodyRef.elem, len(body.rows))
		for rowIndex, row := range body.rows {
			rowRef := elements.rows[row]
			rowRef.require(tb, rowElems[rowIndex])

			cellElems := nestedReorderChildren(tb, rowRef.elem, len(row.cells))
			for cellIndex, cell := range row.cells {
				cellRef := elements.cells[cell]
				cellRef.require(tb, cellElems[cellIndex])

				selectElems := nestedReorderChildren(tb, cellRef.elem, len(cell.selects))
				for selectIndex, selectHandler := range cell.selects {
					selectRef := elements.selects[selectHandler]
					selectRef.require(tb, selectElems[selectIndex])

					optionElems := nestedReorderChildren(tb, selectRef.elem, len(selectHandler.options))
					for optionIndex, option := range selectHandler.options {
						optionRef := elements.options[option]
						optionRef.require(tb, optionElems[optionIndex])
						if option.renderCount != 1 {
							tb.Fatalf("option %q rendered %d times, want once", option.value, option.renderCount)
						}
					}
				}
			}
		}
	}
}

func (elements *nestedReorderElements) updateAll() {
	for _, elem := range elements.updates {
		elem.JawsUpdate()
	}
}

type nestedReorderWireExpectation struct {
	kind what.What
	data string
}

func nestedReorderOrderData[T comparable](items []T, refs map[T]nestedReorderElementRef) string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = refs[item].jid.String()
	}
	return strings.Join(ids, " ")
}

func (elements *nestedReorderElements) wireExpectations(tree *nestedReorderTree) (expectations map[jaws.Jid][]nestedReorderWireExpectation) {
	expectations = make(map[jaws.Jid][]nestedReorderWireExpectation)
	expectations[elements.root.jid] = []nestedReorderWireExpectation{{
		kind: what.Order,
		data: nestedReorderOrderData(tree.bodies, elements.bodies),
	}}
	for _, body := range tree.bodies {
		bodyRef := elements.bodies[body]
		expectations[bodyRef.jid] = []nestedReorderWireExpectation{{
			kind: what.Order,
			data: nestedReorderOrderData(body.rows, elements.rows),
		}}
		for _, row := range body.rows {
			rowRef := elements.rows[row]
			expectations[rowRef.jid] = []nestedReorderWireExpectation{{
				kind: what.Order,
				data: nestedReorderOrderData(row.cells, elements.cells),
			}}
			for _, cell := range row.cells {
				cellRef := elements.cells[cell]
				expectations[cellRef.jid] = []nestedReorderWireExpectation{{
					kind: what.Order,
					data: nestedReorderOrderData(cell.selects, elements.selects),
				}}
				for _, selectHandler := range cell.selects {
					selectRef := elements.selects[selectHandler]
					expectations[selectRef.jid] = []nestedReorderWireExpectation{
						{
							kind: what.Order,
							data: nestedReorderOrderData(selectHandler.options, elements.options),
						},
						{kind: what.Value, data: selectHandler.selected},
					}
				}
			}
		}
	}
	return
}

func (elements *nestedReorderElements) valueWireExpectations(tree *nestedReorderTree) (expectations map[jaws.Jid][]nestedReorderWireExpectation) {
	expectations = make(map[jaws.Jid][]nestedReorderWireExpectation, len(elements.selects))
	for _, body := range tree.bodies {
		for _, row := range body.rows {
			for _, cell := range row.cells {
				for _, selectHandler := range cell.selects {
					selectRef := elements.selects[selectHandler]
					expectations[selectRef.jid] = []nestedReorderWireExpectation{{
						kind: what.Value,
						data: selectHandler.selected,
					}}
				}
			}
		}
	}
	return
}

func nestedReorderDrainWire(t testing.TB, tr *jawstest.TestRequest, probe string) (messages []wire.WsMsg) {
	t.Helper()
	// The inbound wakeup flushes everything already queued before the Alert probe
	// can be handled, so observing the probe is a deterministic batch boundary.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case tr.InCh <- wire.WsMsg{}:
	case <-tr.DoneCh:
		t.Fatal("request loop stopped before accepting the wire wakeup")
	case <-timer.C:
		t.Fatal("timed out sending the wire wakeup")
	}
	select {
	case tr.BcastCh <- wire.Message{What: what.Alert, Data: probe}:
	case <-tr.DoneCh:
		t.Fatal("request loop stopped before accepting the wire probe")
	case <-timer.C:
		t.Fatal("timed out sending the wire probe")
	}
	for {
		select {
		case message, ok := <-tr.OutCh:
			if !ok {
				t.Fatal("request loop stopped before the wire probe arrived")
			}
			if message.What == what.Alert && message.Data == probe {
				return
			}
			messages = append(messages, message)
		case <-tr.DoneCh:
			t.Fatal("request loop stopped before the wire probe arrived")
		case <-timer.C:
			t.Fatal("timed out waiting for the wire probe")
		}
	}
}

func requireNestedReorderWire(t testing.TB, got []wire.WsMsg, want map[jaws.Jid][]nestedReorderWireExpectation) {
	t.Helper()
	var wantOrders, wantValues int
	for _, messages := range want {
		for _, message := range messages {
			switch message.kind {
			case what.Order:
				wantOrders++
			case what.Value:
				wantValues++
			}
		}
	}
	gotByJid := make(map[jaws.Jid][]nestedReorderWireExpectation)
	var orderCount, valueCount int
	for _, message := range got {
		switch message.What {
		case what.Order:
			orderCount++
		case what.Value:
			valueCount++
		default:
			t.Fatalf("unexpected wire mutation for %v: %v %q", message.Jid, message.What, message.Data)
		}
		gotByJid[message.Jid] = append(gotByJid[message.Jid], nestedReorderWireExpectation{
			kind: message.What,
			data: message.Data,
		})
	}
	if orderCount != wantOrders || valueCount != wantValues {
		t.Fatalf("wire operations = %d Order and %d Value, want %d Order and %d Value", orderCount, valueCount, wantOrders, wantValues)
	}
	if len(gotByJid) != len(want) {
		t.Fatalf("wire operation targets = %d, want %d", len(gotByJid), len(want))
	}
	for jid, wantMessages := range want {
		gotMessages := gotByJid[jid]
		if !slices.Equal(gotMessages, wantMessages) {
			t.Errorf("wire operations for %v = %+v, want %+v", jid, gotMessages, wantMessages)
		}
	}
}

// TestNestedContainers_ReorderAtEveryLevelReusesElements verifies value-based
// identity through a valid table/tbody/tr/td/select/option hierarchy.
//
// Reconciliation is parent-local: sibling reorder keeps an Element and moving an
// outer sibling keeps its whole subtree, while moving a UI between distinct parents
// is reparenting and is not covered by this identity guarantee. Every owning Element
// is therefore updated explicitly after its own sibling collection is reversed.
func TestNestedContainers_ReorderAtEveryLevelReusesElements(t *testing.T) {
	tr := newReuseRequest(t)
	tree := newNestedReorderTree(2, 2, 2, 2, 3)
	root := tr.NewElement(NewContainer("table", tree))
	var output strings.Builder
	if err := root.JawsRender(&output, nil); err != nil {
		t.Fatal(err)
	}
	elements := newNestedReorderElements(t, root, tree)
	elements.requireTree(t, tree)

	initial := nestedReorderDrainWire(t, tr, "nested reorder initial render")
	requireNestedReorderWire(t, initial, elements.valueWireExpectations(tree))

	for round := 1; round <= 2; round++ {
		tree.reverse()
		wantWire := elements.wireExpectations(tree)
		elements.updateAll()
		elements.requireTree(t, tree)
		gotWire := nestedReorderDrainWire(t, tr, "nested reorder round "+strconv.Itoa(round))
		requireNestedReorderWire(t, gotWire, wantWire)
	}
}
