package jawstree

import (
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/staticserve"
)

// TestSetup_PrefixVariants verifies that for any prefix form (absolute, relative
// or empty) Setup neither panics nor diverges: every emitted asset URL appears in
// the head HTML and resolves to a registered handler.
func TestSetup_PrefixVariants(t *testing.T) {
	var names []string
	if err := staticserve.WalkDir(assetsFS, "assets", func(_ string, ss *staticserve.StaticServe) error {
		names = append(names, ss.Name)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, prefix := range []string{"/static", "static", ""} {
		t.Run("prefix="+strconv.Quote(prefix), func(t *testing.T) {
			mux := http.NewServeMux()
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()

			if err := jw.Setup(mux.Handle, prefix, Setup); err != nil {
				t.Fatal(err)
			}

			rq := jw.NewRequest(nil)
			var sb strings.Builder
			if err := (ui.RequestWriter{Request: rq, Writer: &sb}).HeadHTML(); err != nil {
				t.Fatal(err)
			}
			head := sb.String()

			for _, name := range names {
				wantURI := staticserve.EnsurePrefixSlash(path.Join(prefix, name))
				if !strings.Contains(head, `"`+wantURI+`"`) {
					t.Errorf("head html missing %q", wantURI)
				}
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, wantURI, nil))
				if rr.Code != http.StatusOK {
					t.Errorf("GET %q (prefix %q) = %d, want 200 (head URL must match a registered handler)", wantURI, prefix, rr.Code)
				}
			}
		})
	}
}

// TestNew_RejectsInvalidID verifies that New rejects ids that cannot identify the
// browser-side root variable and tree instance, while accepting a valid one.
func TestNew_RejectsInvalidID(t *testing.T) {
	var mu deadlock.RWMutex
	for _, id := range []string{"", "my-tree", "tree.1", "a b", "with/slash"} {
		t.Run(strconv.Quote(id), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%q, ...) should panic on an invalid id", id)
				}
			}()
			New(id, ui.NewJsVar(&mu, &Node{}))
		})
	}

	// A valid id must still construct.
	if New("valid_$Id0", ui.NewJsVar(&mu, &Node{})) == nil {
		t.Error("New with a valid id returned nil")
	}
}
