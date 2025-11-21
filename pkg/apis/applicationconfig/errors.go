package applicationconfig

import "errors"

var (
	ErrDNSAliasUsedByOtherApplication = errors.New("dns alias is used by another application")
)
