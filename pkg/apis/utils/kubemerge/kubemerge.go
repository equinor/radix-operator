package kubemerge

import (
	// k8s types rely on v1 omitempty semantics; json/v2 drops empty structs like `emptyDir: {}`.
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func MergePodTemplate(base corev1.PodTemplateSpec, overlay corev1.PodTemplateSpec) (corev1.PodTemplateSpec, error) {
	original, err := json.Marshal(base)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
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
