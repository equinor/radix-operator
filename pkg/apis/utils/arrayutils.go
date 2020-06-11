package utils

// ArrayEqual tells whether a and b contain the same elements at the same index.
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

// ArrayEqualElements tells whether a and b contain the same elements. Elements does not need to be in same index
// A nil argument is equivalent to an empty slice.
func ArrayEqualElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	bmap := make(map[int]string)
	for i, w := range b {
		bmap[i] = w
	}

	for _, v := range a {
		containsV := false
		for i, w := range bmap {
			if w == v {
				containsV = true
				delete(bmap, i)
				break
			}
		}
		if !containsV {
			return false
		}
	}
	return true
}
