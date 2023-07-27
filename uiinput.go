package jaws

type UiInput struct {
	UiHtml
	OnChange func()
}
