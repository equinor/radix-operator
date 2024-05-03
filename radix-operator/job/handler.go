package job

import (
	"context"

	apiconfig "github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Job is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Job
	// is synced successfully
	MessageResourceSynced = "Radix Job synced successfully"
)

// Handler Instance variables
type Handler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	hasSynced   common.HasSynced
	config      *apiconfig.Config
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, config *apiconfig.Config, hasSynced common.HasSynced) Handler {

	handler := Handler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kubeutil,
		hasSynced:   hasSynced,
		config:      config,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(ctx context.Context, namespace, name string, eventRecorder record.EventRecorder) error {
	radixJob, err := t.radixclient.RadixV1().RadixJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// The Job resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			log.Info().Msgf("RadixJob %s/%s in work queue no longer exists", namespace, name)
			return nil
		}

		return err
	}

	syncJob := radixJob.DeepCopy()
	log.Debug().Msgf("Sync job %s", syncJob.Name)

	job := job.NewJob(t.kubeclient, t.kubeutil, t.radixclient, syncJob, t.config)
	err = job.OnSync(ctx)
	if err != nil {
		// TODO: should we record a Warning event when there is an error, similar to batch handler? Possibly do it in common.Controller?
		// Put back on queue
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncJob, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
