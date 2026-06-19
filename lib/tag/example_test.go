package tag_test

import (
	"errors"
	"fmt"

	"github.com/linkdata/jaws/lib/tag"
)

type exampleItem struct {
	Name string
}

func (item *exampleItem) JawsGetTag(tag.Context) any {
	return item
}

func ExampleTagExpand_tagGetter() {
	item := &exampleItem{Name: "row"}
	tags, err := tag.TagExpand(nil, []any{item, tag.Tag("list")})
	if err != nil {
		panic(err)
	}
	fmt.Println(len(tags), tags[0] == item, tags[1] == tag.Tag("list"))

	// Output: 2 true true
}

func ExampleTagExpand_errorsIs() {
	_, err := tag.TagExpand(nil, []int{1})
	fmt.Println(errors.Is(err, tag.ErrNotUsableAsTag))
	fmt.Println(errors.Is(err, tag.ErrNotComparable))

	// Output:
	// true
	// true
}
