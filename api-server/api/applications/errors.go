package applications

import radixhttp "github.com/equinor/radix-operator/api-server/internal/http"

func userShouldBeMemberOfAdminAdGroupError() error {
	return radixhttp.ValidationError("Radix Registration", "User should be a member of at least one admin AD group or their sub-members")
}
