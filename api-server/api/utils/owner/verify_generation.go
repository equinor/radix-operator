package owner

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VerifyCorrectObjectGeneration Returns true if owner is controller of object and annotation matches owner generation
func VerifyCorrectObjectGeneration(owner metav1.Object, object metav1.Object, annotation string) bool {
	controller := metav1.GetControllerOf(object)
	if controller == nil {
		return false
	}

	if controller.UID != owner.GetUID() {
		return false
	}

	observedGeneration, ok := object.GetAnnotations()[annotation]
	return ok && observedGeneration == strconv.Itoa(int(owner.GetGeneration()))
}
