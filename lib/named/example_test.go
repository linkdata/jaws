package named_test

import (
	"fmt"
	"html/template"

	"github.com/linkdata/jaws/lib/named"
)

func ExampleBoolArray_singleSelect() {
	choices := named.NewBoolArray(false).
		Add("red", template.HTML("Red")).
		Add("green", template.HTML("Green"))

	fmt.Println(choices.Set("green", true))
	fmt.Println(choices.Get(), choices.IsChecked("red"), choices.IsChecked("green"))

	choices.Set("missing", true)
	fmt.Println(choices.Get())

	// Output:
	// true
	// green false true
	//
}

func ExampleBoolArray_multiSelect() {
	choices := named.NewBoolArray(true).
		Add("red", template.HTML("Red")).
		Add("green", template.HTML("Green"))

	choices.Set("red", true)
	choices.Set("green", true)
	fmt.Println(choices.JawsGetValues(nil))

	// Output: [red green]
}
