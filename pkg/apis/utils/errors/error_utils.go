package errors

import (
	"fmt"
	"strings"
)

// Concat Creates a single error from a list of errors
func Concat(errs []error) error {
	var errstrings []string
	for _, err := range errs {
		errstrings = append(errstrings, err.Error())
	}

	return fmt.Errorf(strings.Join(errstrings, "\n"))

}
