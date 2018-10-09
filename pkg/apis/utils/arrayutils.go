package utils

// Equal tells whether a and b contain the same elements at the same index.
// A nil argument is equivalent to an empty slice.
func ArrayEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// Equal tells whether a and b contain the same elements. Elements does not need to be in same index
// A nil argument is equivalent to an empty slice.
func ArrayEqualElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		containsV := false
		for _, w := range b {
			if w == v {
				containsV = true
				break
			}
		}
		if !containsV {
			return false
		}
	}
	return true
}
