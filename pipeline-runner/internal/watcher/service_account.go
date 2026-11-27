package watcher

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	k8errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ServiceAccountWatcher Watcher to wait for a service account to be created
type ServiceAccountWatcher interface {
	WaitFor(ctx context.Context, namespace, name string) error
}

type serviceAccountWatcher struct {
	kubeClient  kubernetes.Interface
	waitTimeout time.Duration
}

// NewServiceAccountWatcher Constructor
func NewServiceAccountWatcher(kubeClient kubernetes.Interface, waitTimeout time.Duration) ServiceAccountWatcher {
	return &serviceAccountWatcher{
		kubeClient:  kubeClient,
		waitTimeout: waitTimeout,
	}
}

// WaitFor Waits for the service account to exist
func (watcher serviceAccountWatcher) WaitFor(ctx context.Context, namespace, name string) error {
	log.Ctx(ctx).Info().Msgf("Waiting for service account %s in namespace %s", name, namespace)
	timeoutContext, cancel := context.WithTimeout(ctx, watcher.waitTimeout)
	defer cancel()

	err := wait.PollUntilContextCancel(timeoutContext, time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := watcher.kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8errs.IsNotFound(err) {
				return false, nil // the operator has not yet reconciled the RadixApplication
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return fmt.Errorf("service account %s in namespace %s was not created within %s", name, namespace, watcher.waitTimeout)
		}
		return fmt.Errorf("failed to wait for service account %s in namespace %s: %w", name, namespace, err)
	}

	log.Ctx(ctx).Info().Msgf("Service account %s in namespace %s exists", name, namespace)
	return nil
}
