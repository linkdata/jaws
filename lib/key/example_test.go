package key_test

import (
	"fmt"

	"github.com/linkdata/jaws/lib/key"
)

// ExampleParse shows how a slash splits the key prefix from the trailing path.
func ExampleParse() {
	k, tail := key.Parse("2/noscript")
	fmt.Printf("key=%v tail=%q\n", k, tail)
	// Output: key=2 tail="/noscript"
}

// ExampleKey_String shows that the zero (invalid) Key encodes as an empty string.
func ExampleKey_String() {
	fmt.Printf("%q\n", key.Key(0).String())
	// Output: ""
}
