package httproute

import (
	"errors"
)

var (
	ErrDuplicateHostname    = errors.New("hostname already exists")
	ErrHostnameLabelTooLong = errors.New("hostname label exceeds 63 characters")
)
