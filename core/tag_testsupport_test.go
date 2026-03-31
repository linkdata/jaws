package jaws

import (
	"github.com/linkdata/jaws/core/assets"
	"github.com/linkdata/jaws/core/tags"
	"github.com/linkdata/jaws/core/wire"
)

type TagContext = tags.Context
type TagGetter = tags.TagGetter
type Tag = tags.Tag
type Message = wire.Message
type WsMsg = wire.WsMsg

var MustTagExpand = tags.MustTagExpand
var TagExpand = tags.TagExpand
var TagString = tags.TagString
var JawsKeyAppend = assets.JawsKeyAppend
var JawsKeyString = assets.JawsKeyString
var JawsKeyValue = assets.JawsKeyValue
var JavascriptText = assets.JavascriptText
var JawsCSS = assets.JawsCSS

func (tt *testSelfTagger) JawsGetTag(TagContext) any {
	return tt
}

type testSelfTagger struct{}
