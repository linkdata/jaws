package tag_test

import (
	"errors"
	"fmt"

	"github.com/linkdata/jaws/lib/tag"
)

type exampleItem struct {
	Name string
}

func (item *exampleItem) JawsGetTag() any {
	return item
}

func ExampleTagExpand_tagGetter() {
	item := &exampleItem{Name: "row"}
	tags, err := tag.TagExpand([]any{item, tag.Tag("list")})
	if err != nil {
		panic(err)
	}
	fmt.Println(len(tags), tags[0] == item, tags[1] == tag.Tag("list"))

	// Output: 2 true true
}

// ExampleTagGetter shows the two supported ways to read an object's tags:
// JawsGetTag directly, which returns the raw value, and TagExpand, which flattens and
// validates it into keys. Both are stable for as long as the getter is idempotent.
func ExampleTagGetter() {
	group := &exampleItem{Name: "group"}
	item := &exampleItem{Name: "row"}

	// JawsGetTag is the canonical public accessor and may be called directly.
	fmt.Println(item.JawsGetTag() == item)

	// TagExpand is how to obtain flattened, validated keys.
	keys, err := tag.TagExpand([]any{item, group})
	if err != nil {
		panic(err)
	}
	fmt.Println(len(keys), keys[0] == item, keys[1] == group)

	// Output:
	// true
	// 2 true true
}

func ExampleTagExpand_errorsIs() {
	_, err := tag.TagExpand([]int{1})
	fmt.Println(errors.Is(err, tag.ErrNotUsableAsTag))
	fmt.Println(errors.Is(err, tag.ErrNotComparable))

	// Output:
	// true
	// true
}
