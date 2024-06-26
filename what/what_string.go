// Code generated by "stringer -type=What"; DO NOT EDIT.

package what

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[invalid-0]
	_ = x[Update-1]
	_ = x[Reload-2]
	_ = x[Redirect-3]
	_ = x[Alert-4]
	_ = x[Order-5]
	_ = x[Call-6]
	_ = x[Set-7]
	_ = x[separator-8]
	_ = x[Inner-9]
	_ = x[Delete-10]
	_ = x[Replace-11]
	_ = x[Remove-12]
	_ = x[Insert-13]
	_ = x[Append-14]
	_ = x[SAttr-15]
	_ = x[RAttr-16]
	_ = x[SClass-17]
	_ = x[RClass-18]
	_ = x[Value-19]
	_ = x[Input-20]
	_ = x[Click-21]
	_ = x[Hook-22]
}

const _What_name = "invalidUpdateReloadRedirectAlertOrderCallSetseparatorInnerDeleteReplaceRemoveInsertAppendSAttrRAttrSClassRClassValueInputClickHook"

var _What_index = [...]uint8{0, 7, 13, 19, 27, 32, 37, 41, 44, 53, 58, 64, 71, 77, 83, 89, 94, 99, 105, 111, 116, 121, 126, 130}

func (i What) String() string {
	if i >= What(len(_What_index)-1) {
		return "What(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _What_name[_What_index[i]:_What_index[i+1]]
}
