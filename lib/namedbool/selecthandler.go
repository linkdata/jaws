package namedbool

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type SelectHandler interface {
	jaws.Container
	bind.Setter[string]
}
