package jaws

type Tag struct{ Value interface{} }

func (rq *Request) Tags(params ...interface{}) (tags []Tag) {
	for _, p := range params {
		tags = append(tags, Tag{Value: p})
	}
	return
}
