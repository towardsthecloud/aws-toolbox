package cliutil

// PointerToString safely dereferences a *string, returning "" for nil.
func PointerToString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// PointerToInt32 safely dereferences a *int32, returning 0 for nil.
func PointerToInt32(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

// Ptr returns a pointer to the given value.
func Ptr[T any](value T) *T {
	return &value
}
