package kube

import (
	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type Kube struct {
	kubeClient kubernetes.Interface
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "kube-api"})
}

func New(client kubernetes.Interface) (*Kube, error) {
	kube := &Kube{
		kubeClient: client,
	}
	return kube, nil
}
