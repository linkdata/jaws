package testutil

import (
	"io"

	core "github.com/linkdata/jaws/core"
)

// NoopCoreUI is a minimal core.UI used by tests.
type NoopCoreUI struct{}

// JawsRender implements core.UI.
func (NoopCoreUI) JawsRender(*core.Element, io.Writer, []any) error { return nil }

// JawsUpdate implements core.UI.
func (NoopCoreUI) JawsUpdate(*core.Element) {}
