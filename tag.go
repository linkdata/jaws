package jaws

import (
	"fmt"
	"reflect"
)

type Tag struct{ Value interface{} }

func (rq *Request) Tags(params ...interface{}) (tags []Tag) {
	for _, p := range params {
		tags = append(tags, Tag{Value: p})
	}
	return
}

func TagString(tag interface{}) string {
	if rv := reflect.ValueOf(tag); rv.IsValid() {
		if rv.Kind() == reflect.Pointer {
			return fmt.Sprintf("%T(%p)", tag, tag)
		} else if stringer, ok := tag.(fmt.Stringer); ok {
			return fmt.Sprintf("%T(%s)", tag, stringer.String())
		}
	}
	return fmt.Sprintf("%#v", tag)
}
