package deployment

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	radixinformer "github.com/statoil/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/statoil/radix-operator/radix-operator/common"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// DeployController Instance variables
type DeployController struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "deployment-controller"})
}

// NewDeployController creates a new controller that handles RadixDeployments
func NewDeployController(client kubernetes.Interface, radixClient radixclient.Interface, handler common.Handler) *common.Controller {
	latestResourceVersion, err := getLatestRadixDeploymentResourceVersion(radixClient)
	if err != nil {
		logger.Infof("Error getting latest radix deployment resource version: %v", err)
	} else {
		logger.Infof("Latest RadixDeployment resource version: %s", latestResourceVersion)
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := radixinformer.NewRadixDeploymentInformer(
		radixClient,
		meta_v1.NamespaceAll,
		0,
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			radixDeployment, ok := obj.(*v1.RadixDeployment)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Deployment; instead was %v", obj)
				return
			}

			logger = logger.WithFields(log.Fields{"deploymentName": radixDeployment.ObjectMeta.Name, "deploymentNamespace": radixDeployment.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				if shouldAddToQueue(obj, key, latestResourceVersion) {
					logger.Infof("Added radix deployment: %s", key)
					queue.Add(common.QueueItem{Key: key, Operation: common.Add})
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			radixDeployment, ok := newObj.(*v1.RadixDeployment)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Deployment; instead was %v", newObj)
				return
			}

			logger = logger.WithFields(log.Fields{"deploymentName": radixDeployment.ObjectMeta.Name, "deploymentNamespace": radixDeployment.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(oldObj)
			if err == nil {
				logger.Infof("Updated radix deployment: %s", key)
				queue.Add(common.QueueItem{Key: key, OldObject: oldObj, Operation: common.Update})
			}
		},
		DeleteFunc: func(obj interface{}) {
			radixDeployment, ok := obj.(*v1.RadixDeployment)
			if !ok {
				logger.Errorf("Provided object was not a valid Radix Deployment; instead was %v", obj)
				return
			}

			logger = logger.WithFields(log.Fields{"deploymentName": radixDeployment.ObjectMeta.Name, "deploymentNamespace": radixDeployment.ObjectMeta.Namespace})

			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				logger.Infof("Deleted radix deployment: %s", key)
				queue.Add(common.QueueItem{Key: key, Operation: common.Delete})
			}
		},
	})

	controller := &common.Controller{
		KubeClient:  client,
		RadixClient: radixClient,
		Informer:    informer,
		Queue:       queue,
		Handler:     handler,
		Log:         logger,
	}
	return controller
}

func shouldAddToQueue(obj interface{}, key, latestResourceVersion string) bool {
	radixDeployment, ok := obj.(*v1.RadixDeployment)
	if !ok {
		logger.Error("Provided object was not a valid Radix Deployment; instead was %v", obj)
		return true
	}

	logger = logger.WithFields(log.Fields{"deploymentName": radixDeployment.ObjectMeta.Name, "deploymentNamespace": radixDeployment.ObjectMeta.Namespace})

	radixDeploymentResourceVersion := radixDeployment.ResourceVersion
	if comp := strings.Compare(radixDeploymentResourceVersion, latestResourceVersion); comp <= 0 {
		logger.Infof("Not added radix deployment: %s. The object's resource version is smaller than the last resource version.", key)
		return false
	}

	return true
}

func getLatestRadixDeploymentResourceVersion(radixClient radixclient.Interface) (string, error) {
	radixDeployments, err := radixClient.RadixV1().RadixDeployments("").List(meta_v1.ListOptions{})
	if err == nil {
		var maxResourceVersion string
		maxTime := meta_v1.Time{}
		for _, radixDeployment := range radixDeployments.Items {
			radixDeploymentTimestamp := radixDeployment.ObjectMeta.CreationTimestamp
			if radixDeploymentTimestamp.After(maxTime.Time) {
				maxTime = radixDeploymentTimestamp
				maxResourceVersion = radixDeployment.ResourceVersion
			}
		}
		logger.Infof("Max RadixDeployment timestamp: %v", maxTime)
		logger.Infof("Max RadixDeployment resource version: %s", maxResourceVersion)
		return maxResourceVersion, nil
	}
	return "", err
}
