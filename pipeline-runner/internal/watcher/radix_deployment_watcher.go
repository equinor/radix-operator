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
	WaitForActive(namespace, deploymentName string) error
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
func (watcher radixDeploymentWatcher) WaitForActive(namespace, deploymentName string) error {
	log.Info().Msgf("Waiting for Radix deployment %s to activate", deploymentName)
	if err := watcher.waitFor(func(context.Context) (bool, error) {
		rd, err := watcher.radixClient.RadixV1().RadixDeployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return rd != nil && !rd.Status.ActiveFrom.IsZero(), nil
	}); err != nil {
		return err
	}

	log.Info().Msgf("Radix deployment %s in namespace %s is active", deploymentName, namespace)
	return nil

}

func (watcher radixDeploymentWatcher) waitFor(condition wait.ConditionWithContextFunc) error {
	timoutContext, cancel := context.WithTimeout(context.Background(), watcher.waitTimeout)
	defer cancel()
	return wait.PollUntilContextCancel(timoutContext, time.Second, true, condition)
}
