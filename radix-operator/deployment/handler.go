package deployment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	log "github.com/sirupsen/logrus"

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

// Handler Instance variables
type Handler struct {
	kubeclient              kubernetes.Interface
	radixclient             radixclient.Interface
	prometheusperatorclient monitoring.Interface
	kubeutil                *kube.Kube
	hasSynced               common.HasSynced
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	hasSynced common.HasSynced) Handler {

	handler := Handler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kubeutil,
		hasSynced:               hasSynced,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	rd, err := t.kubeutil.GetRadixDeployment(namespace, name)
	if err != nil {
		// The Deployment resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Radix deployment '%s' in work queue no longer exists", name))
			return nil
		}

		return err
	}
	if deployment.IsRadixDeploymentInactive(rd) {
		log.Debugf("Ignoring RadixDeployment %s/%s as it's inactive.", rd.GetNamespace(), rd.GetName())
		return nil
	}

	syncRD := rd.DeepCopy()
	logger.Debugf("Sync deployment %s", syncRD.Name)

	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), syncRD.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Failed to get RadixRegistartion object: %v", err))
			return nil
		}

		return err
	}

	deployment, err := deployment.NewDeployment(t.kubeclient, t.kubeutil, t.radixclient, t.prometheusperatorclient, radixRegistration, syncRD)
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
