package maps

// GetKeysFromByteMap Returns keys
func GetKeysFromByteMap(mapData map[string][]byte) []string {
	keys := []string{}
	for k := range mapData {
		keys = append(keys, k)
	}

	return keys
}
