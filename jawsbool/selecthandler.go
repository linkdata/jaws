package jawsbool

import "github.com/linkdata/jaws"

type SelectHandler interface {
	jaws.Container
	jaws.Setter[string]
}
