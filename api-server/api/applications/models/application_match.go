package models

import (
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// ApplicationMatch defines a match function that takes a RadixRegistration as parameter and returns a bool indicating if the RR matched the filter or not
type ApplicationMatch func(rr *v1.RadixRegistration) bool

// MatchByNamesFunc returns a ApplicationMatch that checks if the name of a RadixRegistration matches one of the supplied names
func MatchByNamesFunc(names []string) ApplicationMatch {
	return func(rr *v1.RadixRegistration) bool {
		return filterByNames(rr, names)
	}
}

func filterByNames(rr *v1.RadixRegistration, names []string) bool {
	if rr == nil {
		return false
	}

	for _, name := range names {
		if name == rr.Name {
			return true
		}
	}

	return false
}

// MatchByNamesFunc returns a ApplicationMatch that checks if the CloneURL of a RadixRegistration matches sshRepo argument
func MatchBySSHRepoFunc(sshRepo string) ApplicationMatch {
	return func(rr *v1.RadixRegistration) bool {
		return filterBySSHRepo(rr, sshRepo)
	}
}

func filterBySSHRepo(rr *v1.RadixRegistration, sshRepo string) bool {
	return strings.EqualFold(rr.Spec.CloneURL, sshRepo)
}

// MatchAll returns a ApplicationMatch that always returns true
func MatchAll(rr *v1.RadixRegistration) bool {
	return true
}
