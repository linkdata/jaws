package ui

import (
	"time"

	pkg "github.com/linkdata/jaws/jaws"
)

func init() {
	pkg.RegisterRequestWriterWidgets(pkg.RequestWriterWidgetFactory{
		A: func(inner pkg.HTMLGetter) pkg.UI {
			return NewA(inner)
		},
		Button: func(inner pkg.HTMLGetter) pkg.UI {
			return NewButton(inner)
		},
		Checkbox: func(setter pkg.Setter[bool]) pkg.UI {
			return NewCheckbox(setter)
		},
		Container: func(outerHTMLTag string, c pkg.Container) pkg.UI {
			return NewContainer(outerHTMLTag, c)
		},
		Date: func(setter pkg.Setter[time.Time]) pkg.UI {
			return NewDate(setter)
		},
		Div: func(inner pkg.HTMLGetter) pkg.UI {
			return NewDiv(inner)
		},
		Img: func(getter pkg.Getter[string]) pkg.UI {
			return NewImg(getter)
		},
		Label: func(inner pkg.HTMLGetter) pkg.UI {
			return NewLabel(inner)
		},
		Li: func(inner pkg.HTMLGetter) pkg.UI {
			return NewLi(inner)
		},
		Number: func(setter pkg.Setter[float64]) pkg.UI {
			return NewNumber(setter)
		},
		Password: func(setter pkg.Setter[string]) pkg.UI {
			return NewPassword(setter)
		},
		Radio: func(setter pkg.Setter[bool]) pkg.UI {
			return NewRadio(setter)
		},
		Range: func(setter pkg.Setter[float64]) pkg.UI {
			return NewRange(setter)
		},
		Select: func(sh pkg.SelectHandler) pkg.UI {
			return NewSelect(sh)
		},
		Span: func(inner pkg.HTMLGetter) pkg.UI {
			return NewSpan(inner)
		},
		Tbody: func(c pkg.Container) pkg.UI {
			return NewTbody(c)
		},
		Td: func(inner pkg.HTMLGetter) pkg.UI {
			return NewTd(inner)
		},
		Text: func(setter pkg.Setter[string]) pkg.UI {
			return NewText(setter)
		},
		Textarea: func(setter pkg.Setter[string]) pkg.UI {
			return NewTextarea(setter)
		},
		Tr: func(inner pkg.HTMLGetter) pkg.UI {
			return NewTr(inner)
		},
	})
}
