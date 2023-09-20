package jaws

import (
	"fmt"
	"html/template"
	"reflect"
)

type Tag struct{ Value interface{} }

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

func TagExpand(tag interface{}, result []interface{}) []interface{} {
	switch data := tag.(type) {
	case nil:
	case Tag:
		result = append(result, data.Value)
	case readonlyProxy:
		result = append(result, data.Value)
	case atomicProxy:
		result = append(result, data.Value)
	case template.HTML:
		result = append(result, string(data))
	case []Tag:
		for _, v := range data {
			result = append(result, v.Value)
		}
	case []string:
		for _, v := range data {
			result = append(result, v)
		}
	case []template.HTML:
		for _, v := range data {
			result = append(result, string(v))
		}
	case []interface{}:
		for _, v := range data {
			result = TagExpand(v, result)
		}
	default:
		result = append(result, data)
	}
	return result
}
