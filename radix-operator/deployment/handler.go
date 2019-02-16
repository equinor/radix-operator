package deployment

import (
	"fmt"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
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
}

// NewDeployHandler Constructor
func NewDeployHandler(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusperatorclient monitoring.Interface) RadixDeployHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixDeployHandler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kube,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *RadixDeployHandler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
	deployment, err := t.radixclient.RadixV1().RadixDeployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// The Registration resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Radix deployment '%s' in work queue no longer exists", name))
			return nil
		}

		return err
	}

	t.onSync(deployment)
	//eventRecorder.Event(deployment, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

// TODO: Move to deployment domain
func (t *RadixDeployHandler) onSync(radixDeploy *v1.RadixDeployment) error {
	radixRegistration, err := t.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(radixDeploy.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		logger.Infof("Failed to get RadixRegistartion object: %v", err)
		return fmt.Errorf("Failed to get RadixRegistartion object: %v", err)
	}

	deployment, err := deployment.NewDeployment(t.kubeclient, t.radixclient, t.prometheusperatorclient, radixRegistration, radixDeploy)
	if err != nil {
		return err
	}

	isLatest, err := deployment.IsLatestInTheEnvironment()
	if err != nil {
		return fmt.Errorf("Failed to check if RadixDeployment was latest. Error was %v", err)
	}

	if !isLatest {
		return fmt.Errorf("RadixDeployment %s was not the latest. Ignoring", radixDeploy.GetName())
	}

	klog.Infof("Sync deployment %s", radixDeploy.Name)
	//return deployment.OnDeploy()
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixDeployHandler) ObjectCreated(obj interface{}) error {
	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixDeployHandler) ObjectDeleted(key string) error {
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixDeployHandler) ObjectUpdated(objOld, objNew interface{}) error {
	return nil
}
