package job

import (
	"context"
	"sort"
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

func sortRadixJobsByCreatedAsc(radixJobs []radixv1.RadixJob) []radixv1.RadixJob {
	sort.Slice(radixJobs, func(i, j int) bool {
		return isCreatedAfter(&radixJobs[j], &radixJobs[i])
	})
	return radixJobs
}

func sortRadixJobsByCreatedDesc(radixJobs []radixv1.RadixJob) []radixv1.RadixJob {
	sort.Slice(radixJobs, func(i, j int) bool {
		return isCreatedBefore(&radixJobs[j], &radixJobs[i])
	})
	return radixJobs
}

func isCreatedAfter(rj1 *radixv1.RadixJob, rj2 *radixv1.RadixJob) bool {
	rj1Created := rj1.Status.Created
	if rj1Created == nil {
		return true
	}
	rj2Created := rj2.Status.Created
	if rj2Created == nil {
		return false
	}
	return rj1Created.After(rj2Created.Time)
}

func isCreatedBefore(rj1 *radixv1.RadixJob, rj2 *radixv1.RadixJob) bool {
	return !isCreatedAfter(rj1, rj2)
}

func deleteJobPodIfExistsAndNotCompleted(client kubernetes.Interface, namespace, jobName string) error {
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labels.Set{"job-name": jobName}.String()})
	if err != nil {
		return err
	}
	if pods == nil || len(pods.Items) == 0 {
		return nil
	}
	if pods.Items[0].Status.Phase == corev1.PodSucceeded || pods.Items[0].Status.Phase == corev1.PodFailed {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.CoreV1().Pods(namespace).Delete(ctx, pods.Items[0].Name, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil

}
