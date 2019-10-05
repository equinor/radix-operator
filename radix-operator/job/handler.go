package job

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/prometheus/common/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface, hasSynced common.HasSynced) Handler {

	handler := Handler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kubeutil,
		hasSynced:   hasSynced,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	radixJob, err := t.radixclient.RadixV1().RadixJobs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// The Job resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Radix job '%s' in work queue no longer exists", name))
			return nil
		}

		return err
	}

	if job.IsRadixJobDone(radixJob) {
		log.Warnf("Ignoring RadixJob %s/%s as it's inactive.", radixJob.GetNamespace(), radixJob.GetName())
		return nil
	}

	syncJob := radixJob.DeepCopy()
	logger.Debugf("Sync job %s", syncJob.Name)

	job := job.NewJob(t.kubeclient, t.kubeutil, t.radixclient, syncJob)
	err = job.OnSync()
	if err != nil {
		// Put back on queue
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncJob, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
