package ui

//lint:file-ignore SA5008 The embed tags intentionally test encoder behavior across Go versions.

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
)

type oracleHiddenState struct {
	Visible string `json:"visible"`
	Secret  string `json:"secret"`
}

func (state oracleHiddenState) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Visible string `json:"visible"`
	}{Visible: state.Visible})
}

func newLiveJsVarOracleRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
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
	return jw, tr.Request
}

func TestJsVarStoreHiddenNoOpProposalRejects(t *testing.T) {
	jw, rq := newLiveJsVarOracleRequest(t)
	var mu sync.RWMutex
	state := oracleHiddenState{Visible: "public", Secret: "hidden"}
	store := newTestJsVarStore(t, jw, "state", &mu, &state)
	checks := 0
	store.ClientCheck = func(_ *jaws.Element, _ *oracleHiddenState, path string) error {
		checks++
		if path != "visible" {
			return errors.New("secret write denied")
		}
		return nil
	}
	binding, elem, html := renderTestJsVar(t, rq, store)
	if strings.Contains(html, "secret") || strings.Contains(html, "&#34;hidden&#34;") {
		t.Fatalf("hidden field appeared in initial JSON: %q", html)
	}

	for _, tc := range []struct {
		name     string
		proposal string
	}{
		{name: "matching hidden value", proposal: `secret="hidden"`},
		{name: "different hidden value", proposal: `secret="wrong"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := jaws.CallEventHandlers(binding, elem, what.JsVar, tc.proposal); !errors.Is(err, ErrIllegalJsVarPath) {
				t.Fatalf("hidden proposal error = %v, want ErrIllegalJsVarPath", err)
			}
			if state.Secret != "hidden" || checks != 0 {
				t.Fatalf("hidden proposal changed state or ran check: state=%+v checks=%d", state, checks)
			}
		})
	}
}

type oracleAliasMeta struct {
	Owner string `json:"owner"`
}

type oracleAliasState struct {
	Meta  oracleAliasMeta   `json:",embed"`
	Extra map[string]string `json:",embed"`
}

func TestJsVarStoreEmbeddedAliasNoOpProposalRejects(t *testing.T) {
	jw, rq := newLiveJsVarOracleRequest(t)
	var mu sync.RWMutex
	state := oracleAliasState{
		Meta:  oracleAliasMeta{Owner: "alice"},
		Extra: map[string]string{"Meta": "decoy"},
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	visible, err := decodeJsVarJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if object, ok := visible.(map[string]any); !ok || object["Meta"] != "decoy" || object["owner"] != "alice" {
		t.Skipf("encoder does not flatten embedded fields with a map fallback: %s", data)
	}
	store := newTestJsVarStore(t, jw, "state", &mu, &state)
	checks := 0
	store.ClientCheck = func(_ *jaws.Element, _ *oracleAliasState, _ string) error {
		checks++
		return errors.New("alias write denied")
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	for _, tc := range []struct {
		name     string
		proposal string
	}{
		{name: "matching Go alias", proposal: `Meta={"owner":"alice"}`},
		{name: "different Go alias", proposal: `Meta={"owner":"mallory"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := jaws.CallEventHandlers(binding, elem, what.JsVar, tc.proposal); !errors.Is(err, ErrIllegalJsVarPath) {
				t.Fatalf("alias proposal error = %v, want ErrIllegalJsVarPath", err)
			}
			if state.Meta.Owner != "alice" || checks != 0 {
				t.Fatalf("alias proposal changed state or ran check: state=%+v checks=%d", state, checks)
			}
		})
	}
}

type oraclePartialProfile struct {
	Name   string `json:"name"`
	Secret string `json:"-"`
	pin    int
}

type oracleVisibleProfile struct {
	Name string `json:"name"`
}

type oraclePartialState struct {
	Profile oraclePartialProfile            `json:"profile"`
	Users   map[string]oraclePartialProfile `json:"users"`
}

type oraclePartialOutcome struct {
	denied        bool
	checks        int
	sourcePatches []string
	peerPatches   []string
}

func probeJsVarPartialHidden(t *testing.T, state oraclePartialState, check JsVarCheck[oraclePartialState], proposal string) (outcome oraclePartialOutcome, after oraclePartialState) {
	t.Helper()
	jw, source := newLiveJsVarOracleRequest(t)
	peer := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		peer.Close()
		<-peer.DoneCh
	})
	<-peer.ReadyCh
	var mu sync.RWMutex
	store := newTestJsVarStore(t, jw, "state", &mu, &state)
	if !store.partial || !store.hidden {
		t.Fatalf("partial=%v hidden=%v, want both true", store.partial, store.hidden)
	}
	store.ClientCheck = func(elem *jaws.Element, next *oraclePartialState, path string) error {
		outcome.checks++
		return check(elem, next, path)
	}
	binding, elem, html := renderTestJsVar(t, source, store)
	peerBinding, _, peerHTML := renderTestJsVar(t, peer.Request, store)
	const visible = `{&#34;profile&#34;:{&#34;name&#34;:&#34;&#34;},&#34;users&#34;:{&#34;a&#34;:{&#34;name&#34;:&#34;&#34;}}}`
	if !strings.Contains(html, visible) || !strings.Contains(peerHTML, visible) {
		t.Fatalf("unexpected initial JSON %q", html)
	}
	err := jaws.CallEventHandlers(binding, elem, what.JsVar, proposal)
	outcome.denied = errors.Is(err, errOracleDenied)
	if err != nil && !outcome.denied {
		t.Fatalf("proposal error = %v", err)
	}
	if outcome.sourcePatches, err = binding.pendingPatches(); err != nil {
		t.Fatal(err)
	}
	if outcome.peerPatches, err = peerBinding.pendingPatches(); err != nil {
		t.Fatal(err)
	}
	mu.RLock()
	after = state
	mu.RUnlock()
	return
}

var errOracleDenied = errors.New("denied")

func TestJsVarStorePartialHiddenProposalOutcome(t *testing.T) {
	zero := oraclePartialState{Users: map[string]oraclePartialProfile{"a": {}}}
	hiddenStates := []struct {
		name  string
		state oraclePartialState
	}{
		{name: "ignored field", state: oraclePartialState{
			Profile: oraclePartialProfile{Secret: "s3cr3t"},
			Users:   map[string]oraclePartialProfile{"a": {Secret: "s3cr3t"}},
		}},
		{name: "unexported field", state: oraclePartialState{
			Profile: oraclePartialProfile{pin: 1234},
			Users:   map[string]oraclePartialProfile{"a": {pin: 1234}},
		}},
	}
	for _, check := range []struct {
		name  string
		check JsVarCheck[oraclePartialState]
	}{
		{name: "deny", check: func(*jaws.Element, *oraclePartialState, string) error { return errOracleDenied }},
		{name: "allow", check: func(*jaws.Element, *oraclePartialState, string) error { return nil }},
	} {
		for _, proposal := range []string{"profile=null", "users.a=null", "=null"} {
			t.Run(check.name+"/"+proposal, func(t *testing.T) {
				want, _ := probeJsVarPartialHidden(t, zero, check.check, proposal)
				if want.checks != 1 {
					t.Fatalf("zero hidden state: checks = %d, want 1", want.checks)
				}
				for _, hs := range hiddenStates {
					got, after := probeJsVarPartialHidden(t, hs.state, check.check, proposal)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s: outcome %+v differs from zero hidden state %+v", hs.name, got, want)
					}
					if check.name == "allow" {
						var cleared oraclePartialProfile
						switch proposal {
						case "profile=null":
							cleared = after.Profile
						case "users.a=null":
							cleared = after.Users["a"]
						default:
							if !reflect.DeepEqual(after, oraclePartialState{}) {
								t.Fatalf("%s: allowed root state %+v, want zero", hs.name, after)
							}
						}
						if cleared != (oraclePartialProfile{}) {
							t.Fatalf("%s: allowed proposal left %+v, want zero", hs.name, cleared)
						}
					}
				}
			})
		}
	}
}

func TestJsVarStorePartialVisibleNoOpSkipsCheck(t *testing.T) {
	jw, rq := newLiveJsVarOracleRequest(t)
	var mu sync.RWMutex
	state := oracleVisibleProfile{Name: "Ada"}
	store := newTestJsVarStore(t, jw, "state", &mu, &state)
	if !store.partial || store.hidden {
		t.Fatalf("partial=%v hidden=%v, want true false", store.partial, store.hidden)
	}
	checks := 0
	store.ClientCheck = func(*jaws.Element, *oracleVisibleProfile, string) error {
		checks++
		return errOracleDenied
	}
	binding, elem, _ := renderTestJsVar(t, rq, store)
	if err := jaws.CallEventHandlers(binding, elem, what.JsVar, `name="Ada"`); err != nil || checks != 0 {
		t.Fatalf("unchanged visible proposal: err=%v checks=%d, want nil 0", err, checks)
	}
}
