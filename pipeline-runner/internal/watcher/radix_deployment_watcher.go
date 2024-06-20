package watcher

import (
	"context"
	"time"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// RadixDeploymentWatcher Watcher to wait for namespace to be created
type RadixDeploymentWatcher interface {
	WaitForActive(ctx context.Context, namespace, deploymentName string) error
}

// radixDeploymentWatcher Implementation of watcher
type radixDeploymentWatcher struct {
	radixClient radixclient.Interface
	waitTimeout time.Duration
}

// NewRadixDeploymentWatcher Constructor
func NewRadixDeploymentWatcher(radixClient radixclient.Interface, waitTimeout time.Duration) RadixDeploymentWatcher {
	return &radixDeploymentWatcher{
		radixClient,
		waitTimeout,
	}
}

// WaitForActive Waits for the radix deployment gets active
func (watcher radixDeploymentWatcher) WaitForActive(ctx context.Context, namespace, deploymentName string) error {
	log.Ctx(ctx).Info().Msgf("Waiting for Radix deployment %s to activate", deploymentName)
	if err := watcher.waitFor(ctx, func(context.Context) (bool, error) {
		rd, err := watcher.radixClient.RadixV1().RadixDeployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return rd != nil && !rd.Status.ActiveFrom.IsZero(), nil
	}); err != nil {
		return err
	}

	log.Ctx(ctx).Info().Msgf("Radix deployment %s in namespace %s is active", deploymentName, namespace)
	return nil

}

func (watcher radixDeploymentWatcher) waitFor(ctx context.Context, condition wait.ConditionWithContextFunc) error {
	timoutContext, cancel := context.WithTimeout(ctx, watcher.waitTimeout)
	defer cancel()
	return wait.PollUntilContextCancel(timoutContext, time.Second, true, condition)
}
