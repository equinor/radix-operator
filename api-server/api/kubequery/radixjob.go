package kubequery

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixJobs returns all RadixJobs for the specified application.
func GetRadixJobs(ctx context.Context, client radixclient.Interface, appName string) ([]radixv1.RadixJob, error) {
	ns := operatorUtils.GetAppNamespace(appName)
	rjs, err := client.RadixV1().RadixJobs(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return rjs.Items, nil
}

// GetRadixJob returns the RadixJob for the specified application and job name.
func GetRadixJob(ctx context.Context, client radixclient.Interface, appName, jobName string) (*radixv1.RadixJob, error) {
	ns := operatorUtils.GetAppNamespace(appName)
	return client.RadixV1().RadixJobs(ns).Get(ctx, jobName, metav1.GetOptions{})
}
