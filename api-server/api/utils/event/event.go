package event

import (
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	corev1 "k8s.io/api/core/v1"
)

type LastEventWarnings map[string]string

// ConvertToEventWarnings converts Kubernetes Events to EventWarning
func ConvertToEventWarnings(events []corev1.Event) LastEventWarnings {
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreationTimestamp.Before(&events[j].CreationTimestamp)
	})
	return slice.Reduce(events, make(LastEventWarnings), func(acc LastEventWarnings, event corev1.Event) LastEventWarnings {
		if strings.EqualFold(event.Type, "Warning") && strings.EqualFold(event.InvolvedObject.Kind, "Pod") {
			acc[event.InvolvedObject.Name] = event.Message
		}
		return acc
	})
}
