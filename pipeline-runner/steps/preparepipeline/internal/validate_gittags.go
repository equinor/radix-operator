package internal

import (
	"fmt"
	"strings"
)

// GitTagsContainIllegalChars checks if git tags contain illegal characters
func GitTagsContainIllegalChars(gitTags string) error {
	illegalChars := "\"'$"
	strippedGitTags := removeCharacters(gitTags, illegalChars)
	if gitTags != strippedGitTags {
		return fmt.Errorf("git tags %s contained one or more illegal characters %s", gitTags, illegalChars)
	}
	return nil
}

// removeCharacters removes specified characters from input string
func removeCharacters(input string, characters string) string {
	filter := func(r rune) rune {
		if !strings.ContainsRune(characters, r) {
			return r
		}
		return -1
	}
	return strings.Map(filter, input)
}
