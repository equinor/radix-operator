package test

import (
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonclientfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
)

func Setup() (kubernetes.Interface, radixclient.Interface, tektonclient.Interface) {
	kubeclient := kubeclientfake.NewSimpleClientset()
	radixClient := radixclientfake.NewSimpleClientset()
	tektonClient := tektonclientfake.NewSimpleClientset()
	return kubeclient, radixClient, tektonClient
}
