package utils

import (
	"fmt"
	"regexp"
	"strings"
)

const githubRepoURL = "https://github.com/"
const githubSSHURL = "git@github.com:"

var githubRepoPattern = regexp.MustCompile(fmt.Sprintf("%s(.*?)", githubRepoURL))

// GetGithubCloneURLFromRepo Takes a https repo string as input and converts to a git clone url
func GetGithubCloneURLFromRepo(repo string) string {
	if repo == "" {
		return ""
	}

	cloneURL := githubRepoPattern.ReplaceAllString(repo, githubSSHURL)
	cloneURL += ".git"
	return cloneURL
}

// GetGithubRepositoryURLFromCloneURL Takes git clone url as input and converts to a https repo string
func GetGithubRepositoryURLFromCloneURL(cloneURL string) string {
	if cloneURL == "" {
		return ""
	}

	repoName := strings.TrimSuffix(strings.TrimPrefix(cloneURL, githubSSHURL), ".git")
	repo := fmt.Sprintf("%s%s", githubRepoURL, repoName)
	return repo
}

// TernaryString operator
func TernaryString(condition bool, trueValue, falseValue string) string {
	return map[bool]string{true: trueValue, false: falseValue}[condition]
}

// StringPtr returns a pointer to the passed string.
func StringPtr(s string) *string {
	return &s
}

func EqualStringsAsPtr(s1, s2 *string) bool {
	return ((s1 == nil) == (s2 == nil)) && (s1 != nil && strings.EqualFold(*s1, *s2))
}

//EqualStringMaps Compare two string maps
func EqualStringMaps(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key, val1 := range map1 {
		val2, ok := map2[key]
		if !ok || !strings.EqualFold(val1, val2) {
			return false
		}
	}
	return true
}

//EqualStringLists Compare two string lists
func EqualStringLists(list1, list2 []string) bool {
	return len(list1) == len(list2) &&
		equalStringLists(list1, list2)
}

func equalStringLists(list1, list2 []string) bool {
	list1Map := map[string]bool{}
	for _, val := range list1 {
		list1Map[val] = false
	}
	for _, val := range list2 {
		if _, ok := list1Map[val]; !ok {
			return false
		}
	}
	return true
}

func ShortenString(s string, charsToCut int) string {
	return s[:len(s)-charsToCut]
}
