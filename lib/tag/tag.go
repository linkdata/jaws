package tag

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

func ensureUsableTag(tag any) error {
	if tag != nil {
		if t := reflect.TypeOf(tag); t != nil && t.Comparable() {
			return nil
		}
	}
	return NewErrNotUsableAsTag(tag)
}

func appendUniqueTag(result []any, tag any) ([]any, error) {
	for _, existing := range result {
		if existing == tag {
			return result, nil
		}
	}
	result = append(result, tag)
	if len(result) > 100 {
		return result, ErrTooManyTags
	}
	return result, nil
}

func addTag(result []any, tag any) ([]any, error) {
	if err := ensureUsableTag(tag); err != nil {
		return result, err
	}
	return appendUniqueTag(result, tag)
}

func sameActiveNode(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	ta := reflect.TypeOf(a)
	if ta != reflect.TypeOf(b) {
		return false
	}
	if ta.Comparable() {
		return a == b
	}
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	switch va.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return va.Pointer() == vb.Pointer()
	default:
		return false
	}
}

func findActiveIndex(active []any, tag any) int {
	for i := len(active) - 1; i >= 0; i-- {
		if sameActiveNode(active[i], tag) {
			return i
		}
	}
	return -1
}

func addActiveTags(result []any, active []any) ([]any, error) {
	var err error
	for _, node := range active {
		if _, ok := node.(TagGetter); ok {
			if result, err = addTag(result, node); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func expand(depth int, ctx Context, tag any, result []any, active []any) ([]any, error) {
	if depth > 10 || len(result) > 100 {
		return result, ErrTooManyTags
	}
	switch data := tag.(type) {
	case nil:
		return result, nil
	case []Tag:
		var err error
		for _, v := range data {
			if result, err = appendUniqueTag(result, v); err != nil {
				return result, err
			}
		}
		return result, nil
	case TagGetter:
		if idx := findActiveIndex(active, data); idx >= 0 {
			return addActiveTags(result, active[idx:])
		}
		return expand(depth+1, ctx, data.JawsGetTag(ctx), result, append(active, data))
	case []any:
		if idx := findActiveIndex(active, data); idx >= 0 {
			return addActiveTags(result, active[idx:])
		}
		active = append(active, data)
		var err error
		for _, v := range data {
			if result, err = expand(depth+1, ctx, v, result, active); err != nil {
				return result, err
			}
		}
		return result, nil
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
		return result, errIllegalTagType{tag: tag}
	default:
		return addTag(result, data)
	}
	return result, errIllegalTagType{tag: tag}
}

func TagExpand(ctx Context, tag any) ([]any, error) {
	active := make([]any, 0, 12)
	return expand(0, ctx, tag, nil, active)
}

func MustTagExpand(ctx Context, tag any) []any {
	result, err := TagExpand(ctx, tag)
	if err != nil {
		if ctx != nil {
			ctx.MustLog(err)
		} else {
			panic(err)
		}
	}
	return result
}
