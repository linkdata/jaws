package jaws

import (
	"fmt"
	"html/template"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/linkdata/jaws/what"
)

type Params struct {
	tags  []interface{}
	attrs []string
	vp    ValueProxy
	ef    EventFn
	nba   *NamedBoolArray
}

func addTags(tags map[interface{}]struct{}, tag interface{}) {
	if _, ok := tags[tag]; ok {
		return
	}
	switch data := tag.(type) {
	case nil:
		// does nothing
	case Tag:
		addTags(tags, data.Value)
	case []Tag:
		for _, v := range data {
			addTags(tags, v.Value)
		}
	case Template:
		addTags(tags, data.Template)
		addTags(tags, data.Dot)
	case With:
		addTags(tags, data.Dot)
	case []interface{}:
		for _, v := range data {
			addTags(tags, v)
		}
	case []string:
		for _, v := range data {
			tags[v] = struct{}{}
		}
	case []template.HTML:
		for _, v := range data {
			tags[string(v)] = struct{}{}
		}
	case template.HTML:
		tags[string(data)] = struct{}{}
	case interface{}:
		tags[data] = struct{}{}
	default:
		panic(fmt.Errorf("jaws: cant use %T as a tag", data))
	}
}

func unpackValtag(tags map[interface{}]struct{}, valtag interface{}) (vp ValueProxy) {
	switch data := valtag.(type) {
	case nil:
		// does nothing
	case *atomic.Value:
		vp = atomicProxy{Value: data}
		tags[data] = struct{}{}
	case *NamedBoolArray:
		vp = data
		tags[data] = struct{}{}
	case *NamedBool:
		vp = data
		tags[data] = struct{}{}
	case ValueProxy:
		vp = data
		addTags(tags, data)
	case Tag:
		addTags(tags, data)
	case []Tag:
		addTags(tags, data)
	case string:
		vp = readonlyProxy{Value: template.HTML(data)}
	case template.HTML:
		vp = readonlyProxy{Value: data}
	default:
		vp = readonlyProxy{Value: data}
		addTags(tags, data)
	}
	return
}

func NewParams(valtag interface{}, params []interface{}) (up Params) {
	tags := map[interface{}]struct{}{}
	up.vp = unpackValtag(tags, valtag)
	if nba, ok := valtag.(*NamedBoolArray); ok {
		up.nba = nba
	}
	up.process(tags, params)
	up.tags = make([]interface{}, 0, len(tags))
	for tag := range tags {
		up.tags = append(up.tags, tag)
	}
	return
}

func (up *Params) process(tags map[interface{}]struct{}, params []interface{}) {
	for _, p := range params {
		switch data := p.(type) {
		case nil:
			// do nothing
		case EventFn:
			up.ef = data
		case func(*Request, string) error: // ClickFn
			if data != nil {
				up.ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Click {
						err = data(rq, jid)
					}
					return
				}
			}
		case func(*Request, string, string) error: // InputTextFn
			if data != nil {
				up.ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						err = data(rq, jid, val)
					}
					return
				}
			}
		case func(*Request, string, bool) error: // InputBoolFn
			if data != nil {
				up.ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v bool
						if val != "" {
							if v, err = strconv.ParseBool(val); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case func(*Request, string, float64) error: // InputFloatFn
			if data != nil {
				up.ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v float64
						if val != "" {
							if v, err = strconv.ParseFloat(val, 64); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case func(*Request, string, time.Time) error: // InputDateFn
			if data != nil {
				up.ef = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v time.Time
						if val != "" {
							if v, err = time.Parse(ISO8601, val); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case []interface{}:
			up.process(tags, data)
		case string:
			up.attrs = append(up.attrs, data)
		case []string:
			up.attrs = append(up.attrs, data...)
		case template.HTML:
			up.attrs = append(up.attrs, string(data))
		case []template.HTML:
			for _, s := range data {
				up.attrs = append(up.attrs, string(s))
			}
		case interface{}:
			addTags(tags, data)
		default:
			panic(fmt.Errorf("jaws: unhandled parameter type %T", data))
		}
	}
}

func (up *Params) Tags() []interface{} {
	return up.tags
}

func (up *Params) ValueProxy() ValueProxy {
	if up.vp == nil {
		panic("missing jaws.ValueProxy or *atomic.Value")
	}
	return up.vp
}

func (up *Params) Attrs() []string {
	return up.attrs
}
