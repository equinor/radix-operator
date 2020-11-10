package branch

import (
	"regexp"
)

var (
	invalidSeq = regexp.MustCompile("([\\?\\s\\\\\\^~:\\*\\[[:cntrl:]])|([\\./]{2,})|(@\\{)|(/\\.)|^@$|^/|/$|\\.lock$|(\\.lock/)|^$")
)

// IsValidName Checks if a branch name is valid
func IsValidName(branch string) bool {
	return !invalidSeq.MatchString(branch)
}
