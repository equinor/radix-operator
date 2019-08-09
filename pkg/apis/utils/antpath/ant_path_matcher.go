package antpath

import (
	"regexp"
	"strings"
)

const (
	defaultPathSeparator = "/"
)

var (
	wildcardChars   = []string{"*", "**", "?"}
	variablePattern = regexp.MustCompile("^(([A-Za-z0-9][-A-Za-z0-9.]*)?[A-Za-z0-9])?$")
	patternReplacer = strings.NewReplacer("*", ".*", "?", ".")
)

// IsValidBranchPattern Checks that the path is a branch pattern
func IsValidBranchPattern(pattern string) bool {
	if len(pattern) == 0 ||
		string(pattern[0]) == defaultPathSeparator {
		return false
	}

	pattern = patternReplacer.Replace(pattern)
	_, err := regexp.Compile(pattern)
	return err == nil
}

// BranchMatchesPattern Checks that the branch maches the pattern
func BranchMatchesPattern(pattern, branch string) bool {
	pattern = patternReplacer.Replace(pattern)

	branchPattern := regexp.MustCompile(pattern)
	isValidBranch := branchPattern.MatchString(branch)
	return isValidBranch
}

func replaceWildcardChars(token string) string {
	for _, char := range wildcardChars {
		replacer := strings.NewReplacer(char, "")
		token = replacer.Replace(token)
	}

	return token
}
