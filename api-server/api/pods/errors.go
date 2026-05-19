package pods

import (
	"fmt"

	radixhttp "github.com/equinor/radix-common/net/http"
)

// PodNotFoundError Pod not found
func PodNotFoundError(podName string) error {
	return radixhttp.TypeMissingError(fmt.Sprintf("Pod %s not found", podName), nil)
}
