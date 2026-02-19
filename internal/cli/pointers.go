package cli

func pointerToString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func pointerToInt32(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func ptr[T any](value T) *T {
	return &value
}
