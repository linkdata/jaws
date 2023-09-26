package jaws

type UiTagged struct {
	UiHtml
	Tag interface{}
}

func (ui *UiTagged) parseTag(e *Element, tag interface{}) {
	if tag != nil {
		if tagger, ok := tag.(TagGetter); ok {
			ui.Tag = tagger.JawsGetTag(e)
		}
		e.Tag(ui.Tag)
	}
}
