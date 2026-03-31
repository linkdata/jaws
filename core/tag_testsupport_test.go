package jaws

import "github.com/linkdata/jaws/core/tags"

func (tt *testSelfTagger) JawsGetTag(tags.Context) any {
	return tt
}

type testSelfTagger struct{}
