package commithash

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type provider struct {
	kubeUtil *kube.Kube
}

type RadixDeploymentCommit struct {
	RadixDeploymentName string
	CommitHash          string
}

type Provider interface {
	// GetLastCommitHashesForEnvironments  Gets last successful environment Radix deployment commit hashes
	GetLastCommitHashForEnvironment(ctx context.Context, appName, envName string) (RadixDeploymentCommit, error)
}

// NewProvider New instance of the Radix deployment commit hashes provider
func NewProvider(kubeUtil *kube.Kube) Provider {
	return &provider{
		kubeUtil: kubeUtil,
	}
}

func (p *provider) GetLastCommitHashForEnvironment(ctx context.Context, appName, envName string) (RadixDeploymentCommit, error) {
	var deployCommit RadixDeploymentCommit

	activeRD, err := p.getRadixDeploymentsForEnvironment(ctx, appName, envName)
	if err != nil {
		return deployCommit, err
	}

	if activeRD != nil {
		deployCommit.RadixDeploymentName = activeRD.Name

		if commitHash, ok := activeRD.GetAnnotations()[kube.RadixCommitAnnotation]; ok && len(commitHash) > 0 {
			deployCommit.CommitHash = commitHash
		} else {
			deployCommit.CommitHash = activeRD.GetLabels()[kube.RadixCommitLabel]
		}
	}

	return deployCommit, nil
}

func (p *provider) getRadixDeploymentsForEnvironment(ctx context.Context, appName, envName string) (*v1.RadixDeployment, error) {
	environmentNamespace := utils.GetEnvironmentNamespace(appName, envName)
	_, err := p.kubeUtil.KubeClient().CoreV1().Namespaces().Get(context.Background(), environmentNamespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) || errors.IsForbidden(err) {
			return nil, nil //no environment namespace or no role-binding was created for the radix-pipeline - maybe it is new app or env
		}
		return nil, err
	}

	activeRD, err := p.kubeUtil.GetActiveDeployment(ctx, environmentNamespace)
	if err != nil {
		if errors.IsForbidden(err) {
			return nil, nil //no role-binding was created for the radix-pipeline - maybe it is new app or env
		}
		return nil, err
	}
	return activeRD, nil
}
