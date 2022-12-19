package branch

import (
	"regexp"
	"strings"
)

const (
	defaultPathSeparator = "/"
	startOfWord          = "^"
	endOfWord            = "$"
)

var (
	patternReplacer = strings.NewReplacer("/**/", "/.*", "*", "[^/]*", "?", ".")
)

// IsValidPattern Checks that the path is a branch pattern
func IsValidPattern(pattern string) bool {
	if len(pattern) == 0 ||
		string(pattern[0]) == defaultPathSeparator {
		return false
	}

	pattern = patternReplacer.Replace(pattern)
	_, err := regexp.Compile(pattern)
	return err == nil
}

// MatchesPattern Checks that the branch maches the pattern
func MatchesPattern(pattern, branch string) bool {
	pattern = enclosePattern(patternReplacer.Replace(pattern))
	branch = patternReplacer.Replace(branch)

	branchPattern := regexp.MustCompile(pattern)
	isValidBranch := branchPattern.MatchString(branch)
	return isValidBranch
}

func enclosePattern(pattern string) string {
	return startOfWord + pattern + endOfWord
}
