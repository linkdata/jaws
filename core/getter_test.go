package core

import "testing"

func Test_makeGetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Fail()
		}
	}()
	setter2 := MakeGetter[string](123)
	t.Error(setter2)
}

func Test_makeGetter(t *testing.T) {
	setter := MakeGetter[string]("foo")
	if err := setter.(Setter[string]).JawsSet(nil, "bar"); err != ErrValueNotSettable {
		t.Error(err)
	}
}
