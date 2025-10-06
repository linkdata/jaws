package jawstree

type Option int

const (
	SearchEnabled Option = (1 << iota)
	InitiallyExpanded
	MultiSelectEnabled
	ShowSelectAllButton
	ShowInvertSelectionButton
	ShowExpandCollapseAllButtons
	NodeSelectionDisabled
	CascadeSelectChildren
	CheckboxSelectionEnabled
)
