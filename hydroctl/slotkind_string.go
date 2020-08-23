// Code generated by "stringer -type SlotKind"; DO NOT EDIT.

package hydroctl

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[AtLeast-1]
	_ = x[AtMost-2]
	_ = x[Exactly-3]
	_ = x[Continuous-4]
}

const _SlotKind_name = "AtLeastAtMostExactlyContinuous"

var _SlotKind_index = [...]uint8{0, 7, 13, 20, 30}

func (i SlotKind) String() string {
	i -= 1
	if i < 0 || i >= SlotKind(len(_SlotKind_index)-1) {
		return "SlotKind(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _SlotKind_name[_SlotKind_index[i]:_SlotKind_index[i+1]]
}
