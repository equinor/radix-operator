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

type DeployController struct {
	clientset   kubernetes.Interface
	radixclient radixclient.Interface
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	handler     common.Handler
}

func NewDeployController(client kubernetes.Interface, radixClient radixclient.Interface, handler common.Handler) *common.Controller {
	latestResourceVersion, err := getLatestRadixDeploymentResourceVersion(radixClient)
	if err != nil {
		log.Infof("Error getting latest radix deployment resource version: %v", err)
	} else {
		log.Infof("Latest RadixDeployment resource version: %s", latestResourceVersion)
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
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				if shouldAddToQueue(obj, key, latestResourceVersion) {
					log.Infof("Added radix deployment: %s", key)
					queue.Add(common.QueueItem{Key: key, Operation: common.Add})
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(oldObj)
			if err == nil {
				log.Infof("Updated radix deployment: %s", key)
				queue.Add(common.QueueItem{Key: key, Operation: common.Update})
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				log.Infof("Deleted radix deployment: %s", key)
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
		Log:         log.WithField("controller", "deployment"),
	}
	return controller
}

func shouldAddToQueue(obj interface{}, key, latestResourceVersion string) bool {
	radixDeployment, ok := obj.(*v1.RadixDeployment)
	if ok {
		radixDeploymentResourceVersion := radixDeployment.ResourceVersion
		if comp := strings.Compare(radixDeploymentResourceVersion, latestResourceVersion); comp <= 0 {
			log.Infof("Not added radix deployment: %s. The object's resource version is smaller than the last resource version.", key)
			return false
		}
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
		log.Infof("Max RadixDeployment timestamp: %v", maxTime)
		log.Infof("Max RadixDeployment resource version: %s", maxResourceVersion)
		return maxResourceVersion, nil
	}
	return "", err
}
