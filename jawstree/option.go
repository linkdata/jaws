package jawstree

// Option configures a [Tree].
type Option int

const (
	// SearchEnabled enables tree search controls.
	SearchEnabled Option = (1 << iota)
	// InitiallyExpanded renders nodes expanded initially.
	InitiallyExpanded
	// MultiSelectEnabled allows multiple selected nodes.
	MultiSelectEnabled
	// ShowSelectAllButton shows a select-all control.
	ShowSelectAllButton
	// ShowInvertSelectionButton shows an invert-selection control.
	ShowInvertSelectionButton
	// ShowExpandCollapseAllButtons shows expand/collapse-all controls.
	ShowExpandCollapseAllButtons
	// NodeSelectionDisabled disables node selection.
	NodeSelectionDisabled
	// CascadeSelectChildren cascades selection to child nodes.
	CascadeSelectChildren
	// CheckboxSelectionEnabled renders checkbox selection controls.
	CheckboxSelectionEnabled
)
