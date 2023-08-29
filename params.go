package jaws

import (
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

func (up *Params) addString(s string) {
	if len(up.tags) == 0 {
		up.tags = append(up.tags, s)
	} else {
		up.attrs = append(up.attrs, s)
	}
}

func NewParams(params []interface{}) (up Params) {
	up.process(params)
	return
}

func (up *Params) Tags() []interface{} {
	return up.tags
}

func (up *Params) ValueProxy() ValueProxy {
	if up.vp == nil {
		panic("no ValueProxy")
	}
	return up.vp
}

func (up *Params) Attrs() []string {
	return up.attrs
}

func (up *Params) setVp(vp ValueProxy) {
	if up.vp != nil && up.vp != vp {
		panic("jaws: more than one ValueProxy")
	}
	up.vp = vp
}

func (up *Params) process(params []interface{}) {
	for _, p := range params {
		switch data := p.(type) {
		case Tag:
			up.tags = append(up.tags, data.Value)
		case []Tag:
			for _, tag := range data {
				if tag.Value != nil {
					up.tags = append(up.tags, tag.Value)
				}
			}
		case *atomic.Value:
			up.setVp(atomicProxy{Value: data})
			up.tags = append(up.tags, data)
		case *NamedBoolArray:
			up.nba = data
			up.setVp(data)
			up.tags = append(up.tags, data)
		case *NamedBool:
			up.setVp(data)
			up.tags = append(up.tags, data)
			if data.nba != nil {
				up.tags = append(up.tags, data.nba)
			}
		case ValueProxy:
			up.setVp(data)
			up.tags = append(up.tags, data)
		case []string:
			for _, s := range data {
				up.addString(s)
			}
		case string:
			up.addString(data)
		case nil:
			// does nothing
		case EventFn:
			if data != nil {
				up.ef = data
			}
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
			up.process(data)
		default:
			up.tags = append(up.tags, data)
		}
	}
}
