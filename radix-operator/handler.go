package main

import (
	log "github.com/Sirupsen/logrus"
)

type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(obj interface{})
	ObjectUpdated(objOld, objNew interface{})
}

type RadixAppHandler struct {
}

// Init handles any handler initialization
func (t *RadixAppHandler) Init() error {
	log.Info("RadixAppHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *RadixAppHandler) ObjectCreated(obj interface{}) {
	log.Info("RadixAppHandler.ObjectCreated")
}

// ObjectDeleted is called when an object is deleted
func (t *RadixAppHandler) ObjectDeleted(obj interface{}) {
	log.Info("RadixAppHandler.ObjectDeleted")
}

// ObjectUpdated is called when an object is updated
func (t *RadixAppHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("RadixAppHandler.ObjectUpdated")
}
