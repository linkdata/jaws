package jawstree

// Option configures a [Tree].
type Option int

// The bit positions below are wired one-to-one to the literal bit tests in
// jawstreeNew (assets/jawstree.js); do not reorder or insert constants
// mid-block without updating that script.
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

// allOptions is the OR of every defined [Option] bit. [New] rejects any bit outside
// it so an unknown flag surfaces as [ErrInvalidTree] rather than a tree that renders
// with a garbled options value.
const allOptions = SearchEnabled | InitiallyExpanded | MultiSelectEnabled |
	ShowSelectAllButton | ShowInvertSelectionButton | ShowExpandCollapseAllButtons |
	NodeSelectionDisabled | CascadeSelectChildren | CheckboxSelectionEnabled
