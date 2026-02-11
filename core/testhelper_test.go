package core

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
)

func printGoroutineOrigins(t *testing.T) {
	t.Helper()
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	buf = buf[:n]

	lines := bytes.Split(buf, []byte("\n"))
	re := regexp.MustCompile(`\t(.*?):(\d+) \+0x`)
	counts := make(map[string]int)

	for _, line := range lines {
		m := re.FindSubmatch(line)
		if len(m) == 3 {
			loc := fmt.Sprintf("%s:%s", m[1], m[2])
			counts[loc]++
		}
	}

	// Convert to slice for sorting
	type pair struct {
		loc   string
		count int
	}
	var items []pair
	for k, v := range counts {
		if v > 1 { // omit entries with only one goroutine
			items = append(items, pair{k, v})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	for _, item := range items {
		t.Logf("%-50s %4d goroutines\n", item.loc, item.count)
	}
}

type testHelper struct {
	*time.Timer
	*testing.T
}

func newTestHelper(t *testing.T) (th *testHelper) {
	seconds := 3
	if deadlock.Debug {
		seconds *= 10
	}
	th = &testHelper{
		T:     t,
		Timer: time.NewTimer(time.Second * time.Duration(seconds)),
	}
	t.Cleanup(th.Cleanup)
	return
}

func (th *testHelper) Cleanup() {
	th.Timer.Stop()
}

func (th *testHelper) Equal(got, want any) {
	if !testEqual(got, want) {
		th.Helper()
		th.Errorf("\n got %T(%#v)\nwant %T(%#v)\n", got, got, want, want)
	}
}

func (th *testHelper) True(a bool) {
	if !a {
		th.Helper()
		th.Error("not true")
	}
}

func (th *testHelper) NoErr(err error) {
	if err != nil {
		th.Helper()
		th.Error(err)
	}
}

func (th *testHelper) Timeout() {
	th.Helper()
	printGoroutineOrigins(th.T)
	th.Fatalf("timeout")
}

func Test_testHelper(t *testing.T) {
	mustEqual := func(a, b any) {
		if !testEqual(a, b) {
			t.Helper()
			t.Errorf("%#v != %#v", a, b)
		}
	}

	mustNotEqual := func(a, b any) {
		if testEqual(a, b) {
			t.Helper()
			t.Errorf("%#v == %#v", a, b)
		}
	}

	mustEqual(1, 1)
	mustEqual(nil, nil)
	mustEqual(nil, (*testHelper)(nil))
	mustNotEqual(1, nil)
	mustNotEqual(nil, 1)
	mustNotEqual((*testing.T)(nil), 1)
	mustNotEqual(1, 2)
	mustNotEqual((*testing.T)(nil), (*testHelper)(nil))
	mustNotEqual(int(1), int32(1))
}

func testNil(object any) (bool, reflect.Type) {
	if object == nil {
		return true, nil
	}
	value := reflect.ValueOf(object)
	kind := value.Kind()
	return kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil(), value.Type()
}

func testEqual(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	aIsNil, aType := testNil(a)
	bIsNil, bType := testNil(b)
	if !(aIsNil && bIsNil) {
		return false
	}
	return aType == nil || bType == nil || (aType == bType)
}
