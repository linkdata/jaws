package jaws

import "github.com/linkdata/jaws/core/tags"

type Tag = tags.Tag

var ErrTooManyTags = tags.ErrTooManyTags

func TagString(tag any) string {
	return tags.TagString(tag)
}

func TagExpand(rq *Request, tag any) ([]any, error) {
	return tags.TagExpand(rq, tag)
}

func MustTagExpand(rq *Request, tag any) []any {
	return tags.MustTagExpand(rq, tag)
}
