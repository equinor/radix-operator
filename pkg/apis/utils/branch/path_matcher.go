package branch

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
`´	patternReplacer = strings.NewReplacer("*", ".*", "?", ".", "/", "")
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
