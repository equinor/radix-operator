package utils

func IsEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
