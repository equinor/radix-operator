package onpush

import (
	"fmt"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RadixOnPushHandler struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
}

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

func (cli *RadixOnPushHandler) Run(branch, imageTag, appFileName string) error {
	radixApplication, err := getRadixApplication(appFileName)
	if err != nil {
		log.Errorf("failed to get ra from file (%s) for app Error: %v", appFileName, err)
		return err
	}

	appName := radixApplication.Name
	log.Infof("start pipeline build and deploy for %s and branch %s", appName, branch)

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

	err = cli.build(radixRegistration, radixApplication, branch, imageTag)
	if err != nil {
		log.Errorf("failed to build app %s. Error: %v", appName, err)
		return err
	}
	log.Infof("Succeeded: build docker image")

	err = cli.deploy(radixRegistration, radixApplication, imageTag)
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

func getRadixApplication(filename string) (*v1.RadixApplication, error) {
	log.Infof("get radix application yaml from %s", filename)
	radixApp := v1.RadixApplication{}
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file %v Error:  %v ", filename, err)
	}
	err = yaml.Unmarshal(yamlFile, &radixApp)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal: %v", err)
	}

	return &radixApp, nil
}
