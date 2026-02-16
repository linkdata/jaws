package core

import (
	"fmt"
	"html/template"
	"reflect"
)

type Tag string

func TagString(tag any) string {
	if rv := reflect.ValueOf(tag); rv.IsValid() {
		if rv.Kind() == reflect.Pointer {
			return fmt.Sprintf("%T(%p)", tag, tag)
		} else if stringer, ok := tag.(fmt.Stringer); ok {
			return fmt.Sprintf("%T(%s)", tag, stringer.String())
		}
	}
	return fmt.Sprintf("%#v", tag)
}

type errTooManyTags struct{}

func (errTooManyTags) Error() string {
	return "too many tags"
}

var ErrTooManyTags = errTooManyTags{}

func tagExpand(l int, rq *Request, tag any, result []any) ([]any, error) {
	if l > 10 || len(result) > 100 {
		return result, ErrTooManyTags
	}
	switch data := tag.(type) {
	case string:
	case template.HTML:
	case template.HTMLAttr:
	case int:
	case int8:
	case int16:
	case int32:
	case int64:
	case uint:
	case uint8:
	case uint16:
	case uint32:
	case uint64:
	case float32:
	case float64:
	case bool:

	case nil:
		return result, nil
	case []Tag:
		for _, v := range data {
			result = append(result, v)
		}
		return result, nil
	case TagGetter:
		newTag := data.JawsGetTag(rq)
		if reflect.TypeOf(data) == reflect.TypeOf(newTag) {
			if err := newErrNotComparable(newTag); err != nil {
				return result, err
			}
			if data == newTag {
				return append(result, data), nil
			}
		}
		return tagExpand(l+1, rq, newTag, result)
	case []any:
		var err error
		for _, v := range data {
			if result, err = tagExpand(l+1, rq, v, result); err != nil {
				break
			}
		}
		return result, err
	default:
		if err := newErrNotComparable(data); err != nil {
			return result, err
		}
		return append(result, data), nil
	}
	return result, errIllegalTagType{tag: tag}
}

func TagExpand(rq *Request, tag any) ([]any, error) {
	return tagExpand(0, rq, tag, nil)
}

func MustTagExpand(rq *Request, tag any) []any {
	result, err := TagExpand(rq, tag)
	rq.MustLog(err)
	return result
}
