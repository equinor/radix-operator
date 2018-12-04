package onpush

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/application"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	validate "github.com/statoil/radix-operator/pkg/apis/radixvalidators"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RadixOnPushHandler Instance variables
type RadixOnPushHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
}

// Init constructor
func Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface) (RadixOnPushHandler, error) {
	kube, err := kube.New(kubeclient)
	if err != nil {
		return RadixOnPushHandler{}, err
	}

	handler := RadixOnPushHandler{
		kubeclient:  kubeclient,
		radixclient: radixclient,
		kubeutil:    kube,
	}

	return handler, nil
}

// Run Runs the main pipeline
func (cli *RadixOnPushHandler) Run(jobName, branch, commitID, imageTag, appFileName string) error {
	radixApplication, err := utils.GetRadixApplication(appFileName)
	if err != nil {
		log.Errorf("failed to get ra from file (%s) for app Error: %v", appFileName, err)
		return err
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.radixclient, radixApplication)
	if !isRAValid {
		log.Errorf("radix config not valid.")
		for _, err = range errs {
			log.Errorf("%v", err)
		}
		return validate.ConcatErrors(errs)
	}

	application := application.NewApplication(radixApplication)
	branchIsMapped, targetEnvironments := application.IsBranchMappedToEnvironment(branch)

	if !branchIsMapped {
		return fmt.Errorf("Failed to match environment to branch: %s", branch)
	}

	appName := radixApplication.Name
	log.Infof("start pipeline build and deploy for %s and branch %s and commit id %s", appName, branch, commitID)

	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations("default").Get(appName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get RR for app %s. Error: %v", appName, err)
		return err
	}

	err = cli.applyRadixApplication(radixRegistration, radixApplication)
	if err != nil {
		log.Errorf("Failed to apply radix application. %v", err)
		return err
	}

	err = cli.build(jobName, radixRegistration, radixApplication, branch, commitID, imageTag)
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

func (cli *RadixOnPushHandler) applyRadixApplication(radixRegistration *v1.RadixRegistration, radixApplication *v1.RadixApplication) error {
	appNamespace := kube.GetCiCdNamespace(radixRegistration)
	_, err := cli.radixclient.RadixV1().RadixApplications(appNamespace).Create(radixApplication)
	if errors.IsAlreadyExists(err) {
		err = cli.radixclient.RadixV1().RadixApplications(appNamespace).Delete(radixApplication.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete radix application. %v", err)
		}

		_, err = cli.radixclient.RadixV1().RadixApplications(appNamespace).Create(radixApplication)
	}
	if err != nil {
		return fmt.Errorf("failed to apply radix application. %v", err)
	}
	log.Infof("RadixApplication %s saved to ns %s", radixApplication.Name, appNamespace)
	return nil
}
