package deployment

import (
	"fmt"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/prometheus/common/log"

	v1Lister "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	extensionlisters "k8s.io/client-go/listers/extensions/v1beta1"
	rbacListers "k8s.io/client-go/listers/rbac/v1"
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
	rdLister                v1Lister.RadixDeploymentLister
	deploymentLister        extensionlisters.DeploymentLister
	serviceLister           coreListers.ServiceLister
	ingressLister           extensionlisters.IngressLister
	secretLister            coreListers.SecretLister
	roleBindingLister       rbacListers.RoleBindingLister
	hasSynced               common.HasSynced
}

// NewHandler Constructor
func NewHandler(kubeclient kubernetes.Interface,
	radixclient radixclient.Interface,
	prometheusperatorclient monitoring.Interface,
	hasSynced common.HasSynced,
	rdLister v1Lister.RadixDeploymentLister,
	deploymentLister extensionlisters.DeploymentLister,
	serviceLister coreListers.ServiceLister,
	ingressLister extensionlisters.IngressLister,
	secretLister coreListers.SecretLister,
	roleBindingLister rbacListers.RoleBindingLister) Handler {
	kube, _ := kube.New(kubeclient)

	handler := Handler{
		kubeclient:              kubeclient,
		radixclient:             radixclient,
		prometheusperatorclient: prometheusperatorclient,
		kubeutil:                kube,
		hasSynced:               hasSynced,
		rdLister:                rdLister,
		deploymentLister:        deploymentLister,
		serviceLister:           serviceLister,
		ingressLister:           ingressLister,
		secretLister:            secretLister,
		roleBindingLister:       roleBindingLister,
	}

	return handler
}

// Sync Is created on sync of resource
func (t *Handler) Sync(namespace, name string, eventRecorder record.EventRecorder) error {
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
	if deployment.IsRadixDeploymentInactive(rd) {
		log.Warnf("Ignoring RadixDeployment %s/%s as it's inactive.", rd.GetNamespace(), rd.GetName())
		return nil
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

	deployment, err := deployment.NewDeploymentWithLister(
		t.kubeclient,
		t.radixclient,
		t.prometheusperatorclient,
		radixRegistration,
		syncRD,
		t.rdLister,
		t.deploymentLister,
		t.serviceLister,
		t.ingressLister,
		t.secretLister,
		t.roleBindingLister,
	)
	if err != nil {
		return err
	}

	err = deployment.OnSync()
	if err != nil {
		// Put back on queue
		return err
	}

	logger.Infof("########################Done syncing deployment %s", syncRD.Name)
	t.hasSynced(true)
	eventRecorder.Event(syncRD, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
