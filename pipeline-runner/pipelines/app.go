package onpush

import (
	"fmt"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
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
func Init(kubeclient kubernetes.Interface, radixclient radixclient.Interface, prometheusOperatorClient monitoring.Interface) RadixOnPushHandler {
	kube, _ := kube.New(kubeclient)
	handler := RadixOnPushHandler{
		kubeclient:               kubeclient,
		radixclient:              radixclient,
		prometheusOperatorClient: prometheusOperatorClient,
		kubeutil:                 kube,
	}

	return handler
}

// Prepare Runs preparations before build
func (cli *RadixOnPushHandler) Prepare(radixApplication *v1.RadixApplication, branch string) (*v1.RadixRegistration, map[string]bool, error) {
	if validate.RAContainsOldPublic(radixApplication) {
		log.Warnf("component.public is deprecated, please use component.publicPort instead")
	}

	isRAValid, errs := validate.CanRadixApplicationBeInsertedErrors(cli.radixclient, radixApplication)
	if !isRAValid {
		log.Errorf("Radix config not valid.")
		for _, err := range errs {
			log.Errorf("%v", err)
		}
		return nil, nil, validate.ConcatErrors(errs)
	}

	appName := radixApplication.GetName()
	radixRegistration, err := cli.radixclient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})
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

func Run(pipelineInfo model.PipelineInfo) error {
	appName := pipelineInfo.GetAppName()
	branch := pipelineInfo.Branch
	commitID := pipelineInfo.CommitID

	log.Infof("Start pipeline %s for app %s. Branch=%s and commit=%s", pipelineInfo.Type, appName, branch, commitID)

	for _, step := range pipelineInfo.Steps {
		err := step.Run(pipelineInfo)
		if err != nil {
			log.Errorf(step.ErrorMsg(pipelineInfo, err))
			return err
		}
		log.Infof(step.SucceededMsg(pipelineInfo))
	}
	return nil
}
