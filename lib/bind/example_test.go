package bind_test

import (
	"fmt"
	"html/template"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

func ExampleBinder_hooks() {
	var mu sync.Mutex
	value := 1
	var calls []string

	b := bind.New(&mu, &value).
		SetLocked(func(prev bind.Binder[int], elem *jaws.Element, value int) error {
			calls = append(calls, "set-locked")
			return prev.JawsSetLocked(elem, value)
		}).
		GetHTML(func(cur bind.Binder[int], elem *jaws.Element) template.HTML {
			calls = append(calls, "get-html")
			return template.HTML(fmt.Sprintf("<strong>%d</strong>", cur.JawsGetLocked(elem))) // #nosec G203
		}).
		Success(func() {
			calls = append(calls, "success-newest")
		}).
		Success(func() {
			calls = append(calls, "success-oldest")
		})

	if err := b.JawsSet(nil, 2); err != nil {
		panic(err)
	}
	htmlGetter := b.(bind.HTMLGetter)
	fmt.Println(value)
	fmt.Println(htmlGetter.JawsGetHTML(nil))
	fmt.Println(calls)

	// Output:
	// 2
	// <strong>2</strong>
	// [set-locked success-oldest success-newest get-html]
}
