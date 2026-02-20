package httproute

import (
	"errors"
)

var (
	ErrDuplicateHostname = errors.New("hostname already exists")
)
