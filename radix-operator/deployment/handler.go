package deployment

import (
	"fmt"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Deployment is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Deployment
	// is synced successfully
	MessageResourceSynced = "Radix Deployment synced successfully"
)

// RadixDeployHandler Instance variables
type RadixDeployHandler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	kubeutil                *kube.Kube
	hasSynced               common.HasSynced
}

// NewDeployHandler Constructor
func NewDeployHandler(kubeclient kubernetes.Interface,
	radixclient radixclient.Interface, prometheusperatorclient monitoring.Interface, hasSynced common.HasSynced) RadixDeployHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixDeployHandler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kube,
		hasSynced:               hasSynced,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *RadixDeployHandler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	rd, err := t.radixclient.RadixV1().RadixDeployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// The Deployment resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Radix deployment '%s' in work queue no longer exists", name))
			return nil
		}

		return err
	}

	syncRD := rd.DeepCopy()
	logger.Infof("Sync deployment %s", syncRD.Name)

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations().Get(syncRD.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Failed to get RadixRegistartion object: %v", err))
			return nil
		}

		return err
	}

	deployment, err := deployment.NewDeployment(t.kubeclient, t.radixclient, t.prometheusperatorclient, radixRegistration, syncRD)
	if err != nil {
		return err
	}

	err = deployment.OnSync()
	if err != nil {
		// Put back on queue
		return err
	}

	t.hasSynced(true)
	eventRecorder.Event(syncRD, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
