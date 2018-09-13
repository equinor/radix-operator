package kube

import (
	"fmt"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

func GetCiCdNamespace(radixRegistration *radixv1.RadixRegistration) string {
	return fmt.Sprintf("%s-app", radixRegistration.Name)
}
