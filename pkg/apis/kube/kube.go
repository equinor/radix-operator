package kube

import (
	"k8s.io/client-go/kubernetes"
)

type Kube struct {
	kubeClient kubernetes.Interface
}

func New(client kubernetes.Interface) (*Kube, error) {
	kube := &Kube{
		kubeClient: client,
	}
	return kube, nil
}
