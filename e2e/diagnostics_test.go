package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// logRadixJobDiagnostics logs the RadixJob status and the logs of all its pipeline pods.
func logRadixJobDiagnostics(t *testing.T, c client.Client, namespace, jobName string) {
	t.Helper()

	rj := &v1.RadixJob{}
	if err := c.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: jobName}, rj); err != nil {
		t.Logf("failed to get RadixJob %s/%s: %v", namespace, jobName, err)
	} else {
		status, _ := json.MarshalIndent(rj.Status, "", "  ")
		t.Logf("RadixJob %s/%s status:\n%s", namespace, jobName, status)
	}

	logPodLogs(t, namespace, fmt.Sprintf("%s=%s", kube.RadixJobNameLabel, jobName), "")
}

// logRadixOperatorLogs logs radix-operator log lines containing filter.
func logRadixOperatorLogs(t *testing.T, c client.Client, filter string) {
	t.Helper()

	deployment := &appsv1.Deployment{}
	if err := c.Get(t.Context(), client.ObjectKey{Namespace: "radix-system", Name: "radix-operator"}, deployment); err != nil {
		t.Logf("failed to get radix-operator deployment: %v", err)
		return
	}
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		t.Logf("failed to parse radix-operator selector: %v", err)
		return
	}
	logPodLogs(t, deployment.Namespace, selector.String(), filter)
}

// logPodLogs logs all container logs for pods matching labelSelector, keeping only lines containing filter when set.
func logPodLogs(t *testing.T, namespace, labelSelector, filter string) {
	t.Helper()
	ctx := t.Context()

	clientset, err := kubernetes.NewForConfig(testManager.GetConfig())
	if err != nil {
		t.Logf("failed to create clientset: %v", err)
		return
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		t.Logf("failed to list pods in %s with selector %q: %v", namespace, labelSelector, err)
		return
	}
	for _, pod := range pods.Items {
		for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			raw, err := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name}).DoRaw(ctx)
			if err != nil {
				t.Logf("failed to get logs for pod %s/%s container %s: %v", namespace, pod.Name, container.Name, err)
				continue
			}
			logs := string(raw)
			if filter != "" {
				var matched []string
				for line := range strings.SplitSeq(logs, "\n") {
					if strings.Contains(line, filter) {
						matched = append(matched, line)
					}
				}
				logs = strings.Join(matched, "\n")
			}
			t.Logf("logs for pod %s/%s container %s:\n%s", namespace, pod.Name, container.Name, logs)
		}
	}
}
