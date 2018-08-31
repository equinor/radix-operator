package onpush

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixOnPushHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
}

func Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface) RadixOnPushHandler {
	kube, _ := kube.New(kubeclient)

	handler := RadixOnPushHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
	}

	return handler
}

func (cli *RadixOnPushHandler) Run(appName, branch string) {
	// should we have different pipeline types?
	// if yes, should each be a small go script?
	// should run in app namespace, where users has access to read pods, jobs, logs (not secrets)
	// pipeline runner should be registered as a job running in app namespace,
	// pointing to pipeline-runner image, with labels to identify runned pipelines
	log.Infof("start pipeline push for %s and branch %s", appName, branch)
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations("default").Get(appName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get rr for app %s. Error: %v", appName, err)
		os.Exit(1)
	}
	_, err = cli.build(radixRegistration, branch)
	if err != nil {
		log.Errorf("failed to build app %s. Error: %v", appName, err)
		os.Exit(2)
	}

	os.Exit(0)
}
