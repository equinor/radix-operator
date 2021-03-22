package numbers

// Int32Ptr converts an int32 to *int32
func Int32Ptr(n int32) *int32 {
	return &n
}

// Int64Ptr converts an int64 to *int64
func Int64Ptr(n int64) *int64 {
	return &n
}

// IntPtr Helper function to get the pointer of an int
func IntPtr(i int) *int {
	return &i
}
