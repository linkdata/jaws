package ui

import (
	"html/template"
	"slices"
	"testing"

	"github.com/linkdata/jaws"
)

const containerStateOwnedTemplates = `
{{define "state-register-container"}}<div id="{{$.RequestWriter.Register $.Dot.Updater}}"></div>{{end}}
{{define "state-owned-container-child"}}{{$.RequestWriter.Template "div" "owned-leaf" $.Dot}}{{$.RequestWriter.Container "div" $.Dot.Container}}{{end}}
`

type registerContainerDot struct {
	updater Container
}

func (d *registerContainerDot) Updater() Container { return d.updater }

func addContainerStateOwnedTemplates(t *testing.T, jw *jaws.Jaws) {
	t.Helper()
	tmpl := template.Must(template.New("container-state-owned").Parse(containerStateOwnedTemplates))
	if err := jw.AddTemplateLookuper(tmpl); err != nil {
		t.Fatal(err)
	}
}

func containerStateOwnedChildren(t *testing.T, elem *jaws.Element) (children []*jaws.Element) {
	t.Helper()
	st, ok := jaws.ElementState(elem).(*containerState)
	if !ok || st == nil {
		t.Fatalf("element %v state = %T, want *containerState", elem.Jid(), jaws.ElementState(elem))
	}
	st.mu.Lock()
	children = slices.Clone(st.contents)
	st.mu.Unlock()
	return
}

func containerStateOwnedTemplateChildren(t *testing.T, elem *jaws.Element) (children []*jaws.Element) {
	t.Helper()
	st, ok := jaws.ElementState(elem).(*templateState)
	if !ok || st == nil {
		t.Fatalf("element %v state = %T, want *templateState", elem.Jid(), jaws.ElementState(elem))
	}
	st.mu.Lock()
	children = slices.Clone(st.owned)
	st.mu.Unlock()
	return
}

// TestTemplateRegisterContainerReclaimsChildren covers the ownership path whose
// Element stores a Register UI wrapper rather than the Container updater. Recursive
// cleanup must find the Container's children through the Element state slot.
func TestTemplateRegisterContainerReclaimsChildren(t *testing.T) {
	jw, rq := newOwnedRequest(t)
	addContainerStateOwnedTemplates(t, jw)

	provider := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("child"))}}
	dot := &registerContainerDot{updater: NewContainer("div", provider)}
	tmpl := NewTemplate("div", "state-register-container", dot)
	wrapper := renderOwned(t, rq, tmpl)

	const wantRegistered = 3 // Template wrapper, Register Element, Container child.
	if got := countRegistered(t, rq); got != wantRegistered {
		t.Fatalf("registered elements after render = %d, want %d", got, wantRegistered)
	}

	for round := 1; round <= 3; round++ {
		generation := containerStateOwnedTemplateChildren(t, wrapper)
		if len(generation) != 1 {
			t.Fatalf("round %d: Template owns %d Elements, want the Register Element", round, len(generation))
		}
		registerElem := generation[0]
		children := containerStateOwnedChildren(t, registerElem)
		if len(children) != 1 {
			t.Fatalf("round %d: Register Container owns %d children, want 1", round, len(children))
		}
		childElem := children[0]

		tmpl.JawsUpdate(wrapper)

		if !registerElem.Deleted() || rq.GetElementByJid(registerElem.Jid()) != nil {
			t.Fatalf("round %d: previous Register Element %v is still registered", round, registerElem.Jid())
		}
		if !childElem.Deleted() || rq.GetElementByJid(childElem.Jid()) != nil {
			t.Fatalf("round %d: previous Container child %v is still registered", round, childElem.Jid())
		}
		if got := countRegistered(t, rq); got != wantRegistered {
			t.Fatalf("registered elements after %d update(s) = %d, want %d", round, got, wantRegistered)
		}
	}
}

// TestContainerDeletedChildReleasesOwnedDescendants covers both reconciliation
// destinations for an out-of-band deleted child: replacement when it remains wanted,
// and discard when it becomes unused. The child Template owns both another Template
// and a Container, whose own child makes the cleanup recursively state-based.
func TestContainerDeletedChildReleasesOwnedDescendants(t *testing.T) {
	for _, tt := range []struct {
		name string
		keep bool
		want int
	}{
		{name: "remains wanted", keep: true, want: 5},
		{name: "becomes unused", keep: false, want: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			jw, rq := newOwnedRequest(t)
			addContainerStateOwnedTemplates(t, jw)

			nestedProvider := &testContainer{contents: []jaws.UI{NewSpan(testHTMLGetter("nested"))}}
			dot := &ownedDot{container: nestedProvider}
			childUI := NewTemplate("div", "state-owned-container-child", dot)
			outerProvider := &testContainer{contents: []jaws.UI{childUI}}
			outer := NewContainer("div", outerProvider)
			outerElem := renderOwned(t, rq, outer)

			if got := countRegistered(t, rq); got != 5 {
				t.Fatalf("registered elements after render = %d, want 5", got)
			}
			children := containerStateOwnedChildren(t, outerElem)
			if len(children) != 1 {
				t.Fatalf("outer Container owns %d children, want 1", len(children))
			}
			deletedChild := children[0]
			before := registeredJids(t, rq)
			oldDescendants := slices.DeleteFunc(before, func(id jaws.Jid) bool {
				return id == outerElem.Jid() || id == deletedChild.Jid()
			})
			if len(oldDescendants) != 3 {
				t.Fatalf("owned descendant count = %d, want 3", len(oldDescendants))
			}

			rq.DeleteElement(deletedChild)
			if got := countRegistered(t, rq); got != 4 {
				t.Fatalf("registered elements after out-of-band child deletion = %d, want 4", got)
			}
			if !tt.keep {
				outerProvider.contents = nil
			}

			outer.JawsUpdate(outerElem)

			if got := countRegistered(t, rq); got != tt.want {
				t.Fatalf("registered elements after reconciliation = %d, want %d", got, tt.want)
			}
			for _, id := range oldDescendants {
				if elem := rq.GetElementByJid(id); elem != nil {
					t.Errorf("old descendant %v remains registered", id)
				}
			}
			if tt.keep {
				fresh := containerStateOwnedChildren(t, outerElem)
				if len(fresh) != 1 || fresh[0] == deletedChild || fresh[0].Deleted() {
					t.Fatalf("recreated children = %v, want one fresh live Element", fresh)
				}
			} else if got := containerStateOwnedChildren(t, outerElem); len(got) != 0 {
				t.Fatalf("unused outer Container children = %v, want none", got)
			}
		})
	}
}

// TestAppendOwnedByToleratesTypedNilState ensures the ownership walk checks exact
// state types for nil before invoking their cleanup methods.
func TestAppendOwnedByToleratesTypedNilState(t *testing.T) {
	_, rq := newCoreRequest(t)
	for _, tt := range []struct {
		name  string
		state any
	}{
		{name: "container", state: (*containerState)(nil)},
		{name: "template", state: (*templateState)(nil)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			elem := rq.NewElement(NewSpan(testHTMLGetter("child")))
			if err := jaws.SetElementState(elem, tt.state); err != nil {
				t.Fatal(err)
			}
			if owned := appendOwnedBy(nil, elem); len(owned) != 0 {
				t.Fatalf("owned Elements = %v, want none", owned)
			}
		})
	}
}
