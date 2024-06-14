package watcher

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const waitTimeout = 15 * time.Second

// NamespaceWatcher Watcher to wait for namespace to be created
type NamespaceWatcher interface {
	WaitFor(ctx context.Context, namespace string) error
}

// NamespaceWatcherImpl Implementation of watcher
type NamespaceWatcherImpl struct {
	client kubernetes.Interface
}

// NewNamespaceWatcherImpl Constructor
func NewNamespaceWatcherImpl(client kubernetes.Interface) NamespaceWatcherImpl {
	return NamespaceWatcherImpl{
		client,
	}
}

// WaitFor Waits for namespace to appear
func (watcher NamespaceWatcherImpl) WaitFor(ctx context.Context, namespace string) error {
	log.Info().Msgf("Waiting for namespace %s", namespace)
	err := waitForNamespace(ctx, watcher.client, namespace)
	if err != nil {
		return err
	}

	log.Info().Msgf("Namespace %s exists and is active", namespace)
	return nil

}

func waitForNamespace(ctx context.Context, client kubernetes.Interface, namespace string) error {
	timoutContext, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	return wait.PollUntilContextCancel(timoutContext, time.Second, true, func(ctx context.Context) (done bool, err error) {
		ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			if k8errs.IsNotFound(err) || k8errs.IsForbidden(err) {
				return false, nil // the environment namespace or the rolebinding for the cluster-role radix-pipeline-env are not yet created
			}
			return false, err
		}
		if ns != nil && ns.Status.Phase == corev1.NamespaceActive {
			return true, nil
		}
		return false, nil
	})
}
