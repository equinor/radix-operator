package slice

import "strings"

// ContainsString return if a string is contained in the slice
func ContainsString(s []string, e string) bool {
	for _, a := range s {
		if strings.EqualFold(a, e) {
			return true
		}
	}
	return false
}
