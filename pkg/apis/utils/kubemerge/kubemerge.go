package kubemerge

import (
	// k8s types rely on v1 omitempty semantics; json/v2 drops empty structs like `emptyDir: {}`.
	"encoding/json"
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// MergePodTemplate applies overlay onto base using Kubernetes strategic merge patch semantics.
//
// Merged (overlay wins on conflicts):
//   - containers, env, imagePullSecrets and volumes are merged by name
//   - ports are merged by containerPort, volumeMounts by mountPath
//   - maps such as labels, annotations and nodeSelector are merged by key
//   - fields not set in overlay keep their base value
//
// Not merged:
//   - atomic lists such as command, args and tolerations are replaced by overlay
//   - empty overlay values cannot clear base values
//   - setting env value does not remove a base valueFrom; both end up set
func MergePodTemplate(templates ...corev1.PodTemplateSpec) (corev1.PodTemplateSpec, error) {
	if len(templates) == 0 {
		return corev1.PodTemplateSpec{}, errors.New("no templates provided")
	}

	if len(templates) == 1 {
		return templates[0], nil
	}

	base := templates[0]
	for i := 1; i < len(templates); i++ {
		var err error
		base, err = mergeTwoPodTemplates(base, templates[i])
		if err != nil {
			return corev1.PodTemplateSpec{}, err
		}
	}
	return base, nil
}

func mergeTwoPodTemplates(base, overlay corev1.PodTemplateSpec) (corev1.PodTemplateSpec, error) {
	overlay = *overlay.DeepCopy()
	original, err := json.Marshal(base)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	// Fields without omitempty marshal nil as null, which deletes the base value.
	if overlay.Spec.Containers == nil {
		overlay.Spec.Containers = []corev1.Container{}
	}
	// projected.sources is atomic, so an empty list would replace base; reuse base sources instead.
	for i, vol := range overlay.Spec.Volumes {
		if vol.Projected != nil && vol.Projected.Sources == nil {
			overlay.Spec.Volumes[i].Projected.Sources = projectedSources(base, vol.Name)
		}
	}

	patch, err := json.Marshal(overlay)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	// dataStruct carries the patchStrategy/patchMergeKey schema for the merge.
	mergedJSON, err := strategicpatch.StrategicMergePatch(original, patch, corev1.PodTemplateSpec{})
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	var got corev1.PodTemplateSpec
	if err := json.Unmarshal(mergedJSON, &got); err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	return got, nil
}

func projectedSources(base corev1.PodTemplateSpec, volumeName string) []corev1.VolumeProjection {
	for _, vol := range base.Spec.Volumes {
		if vol.Name == volumeName && vol.Projected != nil && vol.Projected.Sources != nil {
			return vol.Projected.Sources
		}
	}
	return []corev1.VolumeProjection{}
}
