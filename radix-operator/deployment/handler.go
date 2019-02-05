package deployment

import (
	"fmt"

	monitoring "github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

// Init handles any handler initialization
func (t *RadixDeployHandler) Init() error {
	logger.Info("RadixDeployHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixDeployHandler) ObjectCreated(obj interface{}) error {
	logger.Info("Deploy object created received.")
	radixDeploy, ok := obj.(*v1.RadixDeployment)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Deployment; instead was %v", obj)
	}

	err := t.processRadixDeployment(radixDeploy)
	if err != nil {
		return err
	}

	return nil
}

// ObjectDeleted is called when an object is deleted
func (t *RadixDeployHandler) ObjectDeleted(key string) error {
	logger.Info("RadixDeployment object deleted.")
	return nil
}

// ObjectUpdated is called when an object is updated
func (t *RadixDeployHandler) ObjectUpdated(objOld, objNew interface{}) error {
	logger.Info("Deploy object updated received.")
	radixDeploy, ok := objNew.(*v1.RadixDeployment)
	if !ok {
		return fmt.Errorf("Provided object was not a valid Radix Deployment; instead was %v", objNew)
	}

	err := t.processRadixDeployment(radixDeploy)
	if err != nil {
		return err
	}

	return nil
}

func (t *RadixDeployHandler) processRadixDeployment(radixDeploy *v1.RadixDeployment) error {
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

	logger.Infof("RadixRegistartion %s exists", radixDeploy.Spec.AppName)
	return deployment.OnDeploy()

}
