package commithash

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type provider struct {
	kubeClient   kubernetes.Interface
	radixClient  radixclient.Interface
	appName      string
	environments []string
}

type RadixDeploymentCommit struct {
	RadixDeploymentName string
	CommitHash          string
}

// EnvCommitHashMap Last commit hashes in environments
type EnvCommitHashMap map[string]RadixDeploymentCommit

type Provider interface {
	// GetLastCommitHashesForEnvironments  Gets last successful environment Radix deployment commit hashes
	GetLastCommitHashesForEnvironments() (EnvCommitHashMap, error)
}

// NewProvider New instance of the Radix deployment commit hashes provider
func NewProvider(kubeClient kubernetes.Interface, radixClient radixclient.Interface, appName string, environments []string) Provider {
	return &provider{
		kubeClient:   kubeClient,
		radixClient:  radixClient,
		appName:      appName,
		environments: environments,
	}
}

func (provider *provider) GetLastCommitHashesForEnvironments() (EnvCommitHashMap, error) {
	envCommitMap := make(EnvCommitHashMap)
	appNamespace := utils.GetAppNamespace(provider.appName)
	jobTypeMap, err := getJobTypeMap(provider.radixClient, appNamespace)
	if err != nil {
		return nil, err
	}
	for _, envName := range provider.environments {
		radixDeployments, err := provider.getRadixDeploymentsForEnvironment(envName)
		if err != nil {
			return nil, err
		}
		envCommitMap[envName] = getLastRadixDeploymentCommitHash(radixDeployments, jobTypeMap)
	}
	return envCommitMap, nil
}

func (provider *provider) getRadixDeploymentsForEnvironment(name string) ([]v1.RadixDeployment, error) {
	environmentNamespace := utils.GetEnvironmentNamespace(provider.appName, name)
	_, err := provider.kubeClient.CoreV1().Namespaces().Get(context.Background(), environmentNamespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) || errors.IsForbidden(err) {
			return nil, nil //no environment namespace or no role-binding was created for the radix-tekton - maybe it is new app or env
		}
		return nil, err
	}
	deployments := provider.radixClient.RadixV1().RadixDeployments(environmentNamespace)
	radixDeploymentList, err := deployments.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			return nil, nil //no role-binding was created for the radix-tekton - maybe it is new app or env
		}
		return nil, err
	}
	return radixDeploymentList.Items, nil
}

func getLastRadixDeploymentCommitHash(radixDeployments []v1.RadixDeployment, jobTypeMap map[string]v1.RadixPipelineType) RadixDeploymentCommit {
	var lastRadixDeployment *v1.RadixDeployment
	for _, radixDeployment := range radixDeployments {
		rd := radixDeployment
		pipeLineType, ok := jobTypeMap[rd.GetLabels()[kube.RadixJobNameLabel]]
		if !ok || pipeLineType != v1.BuildDeploy {
			continue
		}
		if lastRadixDeployment == nil || timeIsBefore(lastRadixDeployment.Status.ActiveFrom, rd.Status.ActiveFrom) {
			lastRadixDeployment = &rd
		}
	}
	radixDeploymentCommit := RadixDeploymentCommit{}
	if lastRadixDeployment != nil {
		radixDeploymentCommit.RadixDeploymentName = lastRadixDeployment.GetName()
		if commitHash, ok := lastRadixDeployment.GetAnnotations()[kube.RadixCommitAnnotation]; ok && len(commitHash) > 0 {
			radixDeploymentCommit.CommitHash = commitHash
		} else {
			radixDeploymentCommit.CommitHash = lastRadixDeployment.GetLabels()[kube.RadixCommitLabel]
		}
	}
	return radixDeploymentCommit
}

func getJobTypeMap(radixClient radixclient.Interface, appNamespace string) (map[string]v1.RadixPipelineType, error) {
	radixJobList, err := radixClient.RadixV1().RadixJobs(appNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	jobMap := make(map[string]v1.RadixPipelineType)
	for _, rj := range radixJobList.Items {
		jobMap[rj.GetName()] = rj.Spec.PipeLineType
	}
	return jobMap, nil
}

func timeIsBefore(time1 metav1.Time, time2 metav1.Time) bool {
	if time1.IsZero() {
		return true
	}
	if time2.IsZero() {
		return false
	}
	return time1.Before(&time2)
}
