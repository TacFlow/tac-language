package types

// ValueTypes are the six value types an `input` may declare (SPEC §5.2).
var ValueTypes = []string{"string", "integer", "number", "boolean", "list", "object"}

// IsValueType reports whether s is one of the six value types (case-sensitive).
func IsValueType(s string) bool {
	for _, v := range ValueTypes {
		if v == s {
			return true
		}
	}
	return false
}
