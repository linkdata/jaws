package ui

//lint:file-ignore SA5008 The embed tags intentionally test encoder behavior across Go versions.

import (
	"encoding/json"
	"errors"
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
			if err := jaws.CallEventHandlers(binding, elem, what.Set, tc.proposal); !errors.Is(err, ErrIllegalJsVarPath) {
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
			if err := jaws.CallEventHandlers(binding, elem, what.Set, tc.proposal); !errors.Is(err, ErrIllegalJsVarPath) {
				t.Fatalf("alias proposal error = %v, want ErrIllegalJsVarPath", err)
			}
			if state.Meta.Owner != "alice" || checks != 0 {
				t.Fatalf("alias proposal changed state or ran check: state=%+v checks=%d", state, checks)
			}
		})
	}
}
