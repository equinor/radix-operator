package accounts

import (
	"errors"
	strings "strings"
)

// Impersonation holds user and group to impersonate
type Impersonation struct {
	User   string
	Groups []string
}

// NewImpersonation Constructor
func NewImpersonation(user string, groups []string) (Impersonation, error) {
	impersonation := Impersonation{
		User:   strings.TrimSpace(user),
		Groups: groups,
	}
	return impersonation, impersonation.isValid()
}

// PerformImpersonation Impersonate user
func (impersonation Impersonation) PerformImpersonation() bool {
	return impersonation.User != "" && len(impersonation.Groups) > 0
}

func (impersonation Impersonation) isValid() error {
	impersonateUserSet := impersonation.User != ""
	impersonateGroupSet := len(impersonation.Groups) > 0

	if (impersonateUserSet && !impersonateGroupSet) ||
		(!impersonateUserSet && impersonateGroupSet) {
		return errors.New("Impersonation cannot be done without both user and group being set")
	}
	return nil
}
