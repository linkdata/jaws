package jawstree

import (
	"errors"
	"io/fs"
	"testing"
)

// fakeEntry is a minimal fs.DirEntry for driving getNodes in tests.
type fakeEntry struct {
	name string
	mode fs.FileMode
}

func (e fakeEntry) Name() string               { return e.name }
func (e fakeEntry) IsDir() bool                { return e.mode.IsDir() }
func (e fakeEntry) Type() fs.FileMode          { return e.mode.Type() }
func (e fakeEntry) Info() (fs.FileInfo, error) { return nil, nil }

// fakeFS is an fs.ReadDirFS whose directory listings and per-directory read
// errors are fully scripted, so getNodes can be exercised without touching disk.
type fakeFS struct {
	dirs map[string][]fs.DirEntry
	errs map[string]error
}

func (f fakeFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
}

func (f fakeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err, ok := f.errs[name]; ok {
		return nil, err
	}
	return f.dirs[name], nil
}

func childNames(n *Node) (names []string) {
	for _, c := range n.Children {
		names = append(names, c.Name)
	}
	return
}

func TestGetNodes_partialTreeOnError(t *testing.T) {
	errBad := errors.New("boom")
	fsys := fakeFS{
		dirs: map[string][]fs.DirEntry{
			".": {
				fakeEntry{name: "file.txt"},
				fakeEntry{name: "good", mode: fs.ModeDir},
				fakeEntry{name: "bad", mode: fs.ModeDir},
				fakeEntry{name: "empty", mode: fs.ModeDir},
			},
			"good":  {fakeEntry{name: "leaf.txt"}},
			"empty": {},
		},
		errs: map[string]error{"bad": errBad},
	}

	parent := &Node{}
	err := getNodes(fsys, parent, ".", nil)
	if !errors.Is(err, errBad) {
		t.Fatalf("getNodes err = %v, want to wrap %v", err, errBad)
	}

	// The unreadable "bad" directory is omitted, but its readable siblings
	// (the file, the populated dir, and the empty dir) are retained.
	got := childNames(parent)
	want := []string{"file.txt", "good", "empty"}
	if len(got) != len(want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("children = %v, want %v", got, want)
		}
	}

	// The readable subdirectory keeps its own child.
	for _, c := range parent.Children {
		if c.Name == "good" {
			if names := childNames(c); len(names) != 1 || names[0] != "leaf.txt" {
				t.Fatalf("good children = %v, want [leaf.txt]", names)
			}
		}
		if c.Name == "empty" && len(c.Children) != 0 {
			t.Fatalf("empty dir should have no children, got %v", childNames(c))
		}
	}
}

func TestRootPanicsOnNilRoot(t *testing.T) {
	assertPanics(t, func() {
		_, _ = Root(nil, nil)
	})
}

func TestGetNodes_keepsReadableDirWithFailingDescendant(t *testing.T) {
	// A directory whose own read succeeds must be kept even when a directory
	// nested below it fails to read; only the unreadable directory is omitted,
	// and the failure is still reported. This is the multi-level case the
	// single-level TestGetNodes_partialTreeOnError does not exercise.
	errBad := errors.New("boom")
	fsys := fakeFS{
		dirs: map[string][]fs.DirEntry{
			".": {
				fakeEntry{name: "sibling.txt"},
				fakeEntry{name: "good", mode: fs.ModeDir},
			},
			"good": {
				fakeEntry{name: "leaf.txt"},
				fakeEntry{name: "bad", mode: fs.ModeDir},
			},
		},
		errs: map[string]error{"good/bad": errBad},
	}

	parent := &Node{}
	err := getNodes(fsys, parent, ".", nil)
	if !errors.Is(err, errBad) {
		t.Fatalf("getNodes err = %v, want to wrap %v", err, errBad)
	}

	// The readable "good" directory survives the failure of its child "bad".
	got := childNames(parent)
	want := []string{"sibling.txt", "good"}
	if len(got) != len(want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("children = %v, want %v", got, want)
		}
	}

	// "good" keeps its readable file; only the unreadable "bad" is omitted.
	for _, c := range parent.Children {
		if c.Name == "good" {
			if names := childNames(c); len(names) != 1 || names[0] != "leaf.txt" {
				t.Fatalf("good children = %v, want [leaf.txt]", names)
			}
		}
	}
}

func TestGetNodes_filterFn(t *testing.T) {
	fsys := fakeFS{
		dirs: map[string][]fs.DirEntry{
			".": {
				fakeEntry{name: "keep.txt"},
				fakeEntry{name: "skip.txt"},
			},
		},
	}

	parent := &Node{}
	filter := func(_ string, ent fs.DirEntry) bool { return ent.Name() != "skip.txt" }
	if err := getNodes(fsys, parent, ".", filter); err != nil {
		t.Fatal(err)
	}
	if got := childNames(parent); len(got) != 1 || got[0] != "keep.txt" {
		t.Fatalf("children = %v, want [keep.txt]", got)
	}
}

func TestGetNodes_readDirError(t *testing.T) {
	errRoot := errors.New("cannot read root")
	fsys := fakeFS{errs: map[string]error{".": errRoot}}
	parent := &Node{}
	if err := getNodes(fsys, parent, ".", nil); !errors.Is(err, errRoot) {
		t.Fatalf("getNodes err = %v, want %v", err, errRoot)
	}
	if len(parent.Children) != 0 {
		t.Fatalf("expected no children on root read error, got %v", childNames(parent))
	}
}

func TestGetNodes_excludesIrregularEntries(t *testing.T) {
	// Entries that are neither regular files nor directories (symlinks, named
	// pipes, devices, ...) are always excluded, even when filterFn accepts them: a
	// symlink could otherwise reference a path outside the os.Root sandbox.
	fsys := fakeFS{
		dirs: map[string][]fs.DirEntry{
			".": {
				fakeEntry{name: "file.txt"},
				fakeEntry{name: "dir", mode: fs.ModeDir},
				fakeEntry{name: "link", mode: fs.ModeSymlink},
				fakeEntry{name: "pipe", mode: fs.ModeNamedPipe},
			},
			"dir": {},
		},
	}

	parent := &Node{}
	acceptAll := func(string, fs.DirEntry) bool { return true }
	if err := getNodes(fsys, parent, ".", acceptAll); err != nil {
		t.Fatal(err)
	}

	// Only the regular file and the directory survive; the symlink and the named
	// pipe are dropped regardless of the accept-all filter.
	got := childNames(parent)
	want := []string{"file.txt", "dir"}
	if len(got) != len(want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("children = %v, want %v", got, want)
		}
	}
}
