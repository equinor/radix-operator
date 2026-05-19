package kubequery

import (
	"context"
	"sort"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixDeploymentsForEnvironments returns all RadixDeployments for the specified application and environments.
// Go routines are used to query RDs per environment, and number of concurrent Go routines is controlled with the parallelism parameter.
func GetRadixDeploymentsForEnvironments(ctx context.Context, client radixclient.Interface, appName string, envNames []string, parallelism int) ([]radixv1.RadixDeployment, error) {
	if len(envNames) == 0 {
		return nil, nil
	}
	ch := make(chan []radixv1.RadixDeployment, len(envNames))
	var g errgroup.Group
	g.SetLimit(parallelism)

	for _, envName := range envNames {
		g.Go(func(envName string) func() error {
			return func() error {
				rds, err := GetRadixDeploymentsForEnvironment(ctx, client, appName, envName)
				if err != nil {
					return err
				}
				ch <- rds
				return nil
			}
		}(envName))
	}

	err := g.Wait()
	close(ch)
	if err != nil {
		return nil, err
	}
	var rdList []radixv1.RadixDeployment
	for rd := range ch {
		rdList = append(rdList, rd...)
	}
	return rdList, nil
}

// GetRadixDeploymentsForEnvironment returns all RadixDeployments for the specified application and environment.
func GetRadixDeploymentsForEnvironment(ctx context.Context, radixClient radixclient.Interface, appName, envName string) ([]radixv1.RadixDeployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	rds, err := radixClient.RadixV1().RadixDeployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return rds.Items, nil
}

// GetRadixDeploymentsMapForEnvironment returns all RadixDeployments for the specified application and environment as a map by names.
func GetRadixDeploymentsMapForEnvironment(ctx context.Context, radixClient radixclient.Interface, appName, envName string) (map[string]radixv1.RadixDeployment, error) {
	radixDeployments, err := GetRadixDeploymentsForEnvironment(ctx, radixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	return slice.Reduce(radixDeployments, make(map[string]radixv1.RadixDeployment), func(acc map[string]radixv1.RadixDeployment, radixDeployment radixv1.RadixDeployment) map[string]radixv1.RadixDeployment {
		acc[radixDeployment.Name] = radixDeployment
		return acc
	}), nil
}

// GetRadixDeploymentByName returns a RadixDeployment for an application and namespace
func GetRadixDeploymentByName(ctx context.Context, radixClient radixclient.Interface, appName, envName, deploymentName string) (*radixv1.RadixDeployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	return radixClient.RadixV1().RadixDeployments(ns).Get(ctx, deploymentName, metav1.GetOptions{})
}

// GetLatestRadixDeployment returns the last active Radix Deployment found in environment, will be nil if not found
func GetLatestRadixDeployment(ctx context.Context, radixClient radixclient.Interface, appName, envName string) (*radixv1.RadixDeployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	rdList, err := radixClient.RadixV1().RadixDeployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(rdList.Items) == 0 {
		return nil, nil
	}

	rds := sortRdsByActiveFromDesc(rdList.Items)
	return &rds[0], nil
}

func sortRdsByActiveFromDesc(rds []radixv1.RadixDeployment) []radixv1.RadixDeployment {
	sort.Slice(rds, func(i, j int) bool {
		if rds[j].Status.ActiveFrom.IsZero() {
			return true
		}

		if rds[i].Status.ActiveFrom.IsZero() {
			return false
		}
		return rds[j].Status.ActiveFrom.Before(&rds[i].Status.ActiveFrom)
	})
	return rds
}
