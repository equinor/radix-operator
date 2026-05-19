package metrics

import (
	"context"
	"math"

	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultDuration = "24h"
)

type LabeledResults struct {
	Value       float64
	Environment string
	Component   string
	Pod         string
}
type Client interface {
	// GetCpuRequests returns a list of all pods with their CPU requets. The envName can be empty to return all environments.
	GetCpuRequests(ctx context.Context, appName, envName string, compNames []string) ([]LabeledResults, error)
	// GetCpuAverage returns a list of all pods with their average CPU usage. The envName can be empty to return all environments.
	GetCpuAverage(ctx context.Context, appName, envName string, compNames []string, duration string) ([]LabeledResults, error)
	// GetMemoryRequests returns a list of all pods with their Memory requets. The envName can be empty to return all environments.
	GetMemoryRequests(ctx context.Context, appName, envName string, compNames []string) ([]LabeledResults, error)
	// GetMemoryMaximum returns a list of all pods with their maximum Memory usage. The envName can be empty to return all environments.
	GetMemoryMaximum(ctx context.Context, appName, envName string, compNames []string, duration string) ([]LabeledResults, error)
}

type Handler struct {
	client Client
}

// NewHandler Constructor for Prometheus handler
func NewHandler(client Client) *Handler {
	return &Handler{
		client: client,
	}
}

// GetReplicaResourcesUtilization Get used resources for the application. envName is optional. Will fallback to all copmonent environments to the application.
func (pc *Handler) GetReplicaResourcesUtilization(ctx context.Context, radixClient versioned.Interface, appName, envName string) (*applicationModels.ReplicaResourcesUtilizationResponse, error) {
	application, err := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var compNames []string
	for _, comp := range application.Spec.Components {
		compNames = append(compNames, comp.Name)
	}

	utilization := applicationModels.NewPodResourcesUtilizationResponse()

	results, err := pc.client.GetCpuRequests(ctx, appName, envName, compNames)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetCpuRequests(result.Environment, result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetCpuAverage(ctx, appName, envName, compNames, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetCpuAverage(result.Environment, result.Component, result.Pod, math.Round(result.Value*1e6)/1e6)
	}

	results, err = pc.client.GetMemoryRequests(ctx, appName, envName, compNames)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetMemoryRequests(result.Environment, result.Component, result.Pod, math.Round(result.Value))
	}

	results, err = pc.client.GetMemoryMaximum(ctx, appName, envName, compNames, DefaultDuration)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		utilization.SetMemoryMaximum(result.Environment, result.Component, result.Pod, math.Round(result.Value))
	}

	return utilization, nil
}
