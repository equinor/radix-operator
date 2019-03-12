package onpush

import (
	"fmt"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
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

// LoadConfigFromFile loads radix config from appFileName
func LoadConfigFromFile(appFileName string) (*v1.RadixApplication, error) {
	radixApplication, err := utils.GetRadixApplication(appFileName)
	if err != nil {
		log.Errorf("Failed to get ra from file (%s) for app Error: %v", appFileName, err)
		return nil, err
	}

	return radixApplication, nil
}

// Prepare Runs preparations before build
func (cli *RadixOnPushHandler) Prepare(radixApplication *v1.RadixApplication, branch string) (*v1.RadixRegistration, map[string]bool, error) {
	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.radixclient, radixApplication)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		for _, err := range errs {
			log.Errorf("%v", err)
		}
		return nil, nil, validate.ConcatErrors(errs)
	}

	appName := radixApplication.Name
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get RR for app %s. Error: %v", appName, err)
		return nil, nil, err
	}

	applicationConfig, err := application.NewApplicationConfig(cli.kubeclient, cli.radixclient, radixRegistration, radixApplication)
	if err != nil {
		return nil, nil, err
	}

	err = applicationConfig.ApplyConfigToApplicationNamespace()
	if err != nil {
		log.Errorf("Failed to apply radix application. %v", err)
		return nil, nil, err
	}

	branchIsMapped, targetEnvironments := applicationConfig.IsBranchMappedToEnvironment(branch)

	if !branchIsMapped {
		errMsg := fmt.Sprintf("Failed to match environment to branch: %s", branch)
		log.Warnf(errMsg)
		return nil, nil, fmt.Errorf(errMsg)
	}

	return radixRegistration, targetEnvironments, nil
}

// Run Runs the main pipeline
func (cli *RadixOnPushHandler) Run(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication,
	targetEnvironments map[string]bool, jobName, branch, commitID, imageTag, useCache string) error {

	appName := radixApplication.Name

	log.Infof("Start pipeline build and deploy for %s and branch %s and commit id %s", appName, branch, commitID)
	err := cli.build(jobName, radixRegistration, radixApplication, branch, commitID, imageTag, useCache)
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
