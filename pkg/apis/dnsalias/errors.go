package dnsalias

import (
	"errors"
)

var (
	ErrComponentDoesNotExist = errors.New("component does not exist")
	ErrComponentIsNotPublic  = errors.New("component is not public")
)
