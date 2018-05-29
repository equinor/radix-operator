package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/brigade"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
)

type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(obj interface{})
	ObjectUpdated(objOld, objNew interface{})
}

type RadixAppHandler struct {
	clientset kubernetes.Interface
	brigade   brigade.BrigadeGateway
}

// Init handles any handler initialization
func (t *RadixAppHandler) Init() error {
	log.Info("RadixAppHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixAppHandler) ObjectCreated(obj interface{}) {
	log.Info("RadixAppHandler.ObjectCreated")
	radixApp, ok := obj.(*v1.RadixApplication)
	if !ok {
		log.Error("Provided object was not a valid Radix Application")
		return
	}
	t.brigade.EnsureProject(radixApp)
}

// ObjectDeleted is called when an object is deleted
func (t *RadixAppHandler) ObjectDeleted(obj interface{}) {
	log.Info("RadixAppHandler.ObjectDeleted")
	radixApp, ok := obj.(*v1.RadixApplication)
	if ok {
		t.brigade.DeleteProject(radixApp)
	} else {
		t.brigade.DeleteProject(&v1.RadixApplication{})
	}
}

// ObjectUpdated is called when an object is updated
func (t *RadixAppHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("RadixAppHandler.ObjectUpdated")
	t.brigade.EnsureProject(objNew.(*v1.RadixApplication))
}
