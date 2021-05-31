package maps

// GetKeysFromByteMap Returns keys
func GetKeysFromByteMap(mapData map[string][]byte) []string {
	keys := []string{}
	for k := range mapData {
		keys = append(keys, k)
	}

	return keys
}

// Merge two maps, preferring the right over the left
func MergeStringMaps(left, right map[string]string) map[string]string {
	result := make(map[string]string)

	for key, rVal := range right {
		result[key] = rVal
	}

	for key, lVal := range left {
		if _, present := right[key]; !present {
			result[key] = lVal
		}
	}

	return result
}
