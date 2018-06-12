package main

import (
	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type RadixDeployHandler struct {
	clientset kubernetes.Interface
}

// Init handles any handler initialization
func (t *RadixDeployHandler) Init() error {
	log.Info("RadixDeployHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixDeployHandler) ObjectCreated(obj interface{}) {
	log.Info("Deploy object created.")
}

// ObjectDeleted is called when an object is deleted
func (t *RadixDeployHandler) ObjectDeleted(key string) {
	log.Info("Deploy object deleted.")
}

// ObjectUpdated is called when an object is updated
func (t *RadixDeployHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("Deploy object updated.")
}
