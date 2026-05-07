package applications

import radixhttp "github.com/equinor/radix-common/net/http"

func userShouldBeMemberOfAdminAdGroupError() error {
	return radixhttp.ValidationError("Radix Registration", "User should be a member of at least one admin AD group or their sub-members")
}
