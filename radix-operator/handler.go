package main

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/brigade"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
)

type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(key string)
	ObjectUpdated(objOld, objNew interface{})
}

type RadixAppHandler struct {
	clientset kubernetes.Interface
	brigade   *brigade.BrigadeGateway
}

// Init handles any handler initialization
func (t *RadixAppHandler) Init() error {
	log.Info("RadixAppHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixAppHandler) ObjectCreated(obj interface{}) {
	radixApp, ok := obj.(*v1.RadixApplication)
	if !ok {
		log.Errorf("Provided object was not a valid Radix Application; instead was %v", obj)
		return
	}

	kube, _ := kube.New(t.clientset)
	err := kube.CreateEnvironment(radixApp, "meta")

	for _, e := range radixApp.Spec.Environment {
		err := kube.CreateEnvironment(radixApp, e.Name)
		if err != nil {
			log.Errorf("Failed to create environment: %v", err)
		}
	}

	kube.CreateRoleBindings(radixApp)

	err = t.brigade.EnsureProject(radixApp)
	if err != nil {
		log.Errorf("Failed to create project: %v", err)
	}
}

// ObjectDeleted is called when an object is deleted
func (t *RadixAppHandler) ObjectDeleted(key string) {
	if key == "" {
		log.Errorf("Cannot delete - missing key")
	}
	str := strings.Split(key, "/")
	err := t.brigade.DeleteProject(str[1], str[0])
	if err != nil {
		log.Errorf("Failed to delete project: %v", err)
	}
}

// ObjectUpdated is called when an object is updated
func (t *RadixAppHandler) ObjectUpdated(objOld, objNew interface{}) {
	err := t.brigade.EnsureProject(objNew.(*v1.RadixApplication))
	if err != nil {
		log.Errorf("Failed to create/update project: %v", err)
	}
}
