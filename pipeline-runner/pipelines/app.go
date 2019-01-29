package onpush

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RadixOnPushHandler Instance variables
type RadixOnPushHandler struct {
	kubeclient               kubernetes.Interface
	radixclient              radixclient.Interface
	prometheusOperatorClient monitoring.Interface
	kubeutil                 *kube.Kube
}

// Init constructor
func Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface) (RadixOnPushHandler, error) {
	kube, err := kube.New(kubeclient)
	if err != nil {
		return RadixOnPushHandler{}, err
	}

	handler := RadixOnPushHandler{
		kubeclient:               kubeclient,
		radixclient:              radixclient,
		prometheusOperatorClient: prometheusOperatorClient,
		kubeutil:                 kube,
	}

	return handler, nil
}

// Run Runs the main pipeline
func (cli *RadixOnPushHandler) Run(jobName, branch, commitID, imageTag, appFileName, useCache string) error {
	radixApplication, err := utils.GetRadixApplication(appFileName)
	if err != nil {
		log.Errorf("Failed to get ra from file (%s) for app Error: %v", appFileName, err)
		return err
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.radixclient, radixApplication)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		for _, err = range errs {
			log.Errorf("%v", err)
		}
		return validate.ConcatErrors(errs)
	}

	appName := radixApplication.Name
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RR for app %s. Error: %v", appName, err)
		return err
	}

	application, err := application.NewApplication(cli.kubeclient, cli.radixclient, radixRegistration, radixApplication)
	if err != nil {
		return err
	}

	branchIsMapped, targetEnvironments := application.IsBranchMappedToEnvironment(branch)

	if !branchIsMapped {
		errMsg := fmt.Sprintf("Failed to match environment to branch: %s", branch)
		log.Warnf(errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Infof("start pipeline build and deploy for %s and branch %s and commit id %s", appName, branch, commitID)

	err = application.ApplyConfigToApplicationNamespace()
	if err != nil {
		log.Errorf("Failed to apply radix application. %v", err)
		return err
	}

	err = cli.build(jobName, radixRegistration, radixApplication, branch, commitID, imageTag, useCache)
	if err != nil {
		log.Errorf("failed to build app %s. Error: %v", appName, err)
		return err
	}
	log.Infof("Succeeded: build docker image")

	_, err = cli.Deploy(jobName, radixRegistration, radixApplication, imageTag, branch, commitID, targetEnvironments)
	if err != nil {
		log.Errorf("failed to deploy app %s. Error: %v", appName, err)
		return err
	}
	log.Infof("Succeeded: deploy application")
	return nil
}
