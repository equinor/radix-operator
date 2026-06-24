package predicate

import (
	"testing"

	"github.com/equinor/radix-operator/api-server/api/utils/labelselector"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_IsPodForComponent(t *testing.T) {
	sut := IsPodForComponent("app1", "comp1")
	assert.True(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: radixlabels.Merge(radixlabels.ForApplicationName("app1"), radixlabels.ForComponentName("comp1"))}}))
	assert.False(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: radixlabels.Merge(radixlabels.ForApplicationName("app2"), radixlabels.ForComponentName("comp1"))}}))
	assert.False(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: radixlabels.Merge(radixlabels.ForApplicationName("app1"), radixlabels.ForComponentName("comp2"))}}))
}

func Test_IsPodForAuxComponent(t *testing.T) {
	sut := IsPodForAuxComponent("app1", "comp1", "aux1")
	assert.True(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app1", "comp1", "aux1")}}))
	assert.False(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app2", "comp1", "aux1")}}))
	assert.False(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app1", "comp2", "aux1")}}))
	assert.False(t, sut(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app1", "comp1", "aux2")}}))
}

func Test_IsDeploymentForAuxComponent(t *testing.T) {
	sut := IsDeploymentForAuxComponent("app1", "comp1", "aux1")

	assert.True(t, sut(appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app1", "comp1", "aux1")}}))
	assert.False(t, sut(appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app2", "comp1", "aux1")}}))
	assert.False(t, sut(appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app1", "comp2", "aux1")}}))
	assert.False(t, sut(appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Labels: labelselector.ForAuxiliaryResource("app1", "comp1", "aux2")}}))
}

func Test_IsHpaForComponent(t *testing.T) {
	sut := IsHpaForComponent("app1", "comp1")
	assert.True(t, sut(autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Labels: radixlabels.Merge(radixlabels.ForApplicationName("app1"), radixlabels.ForComponentName("comp1"))}}))
	assert.False(t, sut(autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Labels: radixlabels.Merge(radixlabels.ForApplicationName("app2"), radixlabels.ForComponentName("comp1"))}}))
	assert.False(t, sut(autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Labels: radixlabels.Merge(radixlabels.ForApplicationName("app1"), radixlabels.ForComponentName("comp2"))}}))
}

func Test_IsSecretForSecretStoreProviderClass(t *testing.T) {
	assert.True(t, IsSecretForSecretStoreProviderClass(corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{secretStoreCsiManagedLabel: "true"}}}))
	assert.False(t, IsSecretForSecretStoreProviderClass(corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{secretStoreCsiManagedLabel: "false"}}}))
	assert.False(t, IsSecretForSecretStoreProviderClass(corev1.Secret{}))
}
