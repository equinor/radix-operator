package utils

import (
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
)

// GetFirstTriggerByType returns the trigger spec for the given name  and type or nil if not found
func GetFirstTriggerByType(scaledObject *v1alpha1.ScaledObject, triggerType string) *v1alpha1.ScaleTriggers {
	for _, trigger := range scaledObject.Spec.Triggers {
		if trigger.Type == triggerType {
			return &trigger
		}
	}
	return nil
}
