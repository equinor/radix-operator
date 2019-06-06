package kube

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// Radix Labels
const (
	RadixAppLabel             = "radix-app"
	RadixEnvLabel             = "radix-env"
	RadixComponentLabel       = "radix-component"
	RadixJobNameLabel         = "radix-job-name"
	RadixBuildLabel           = "radix-build"
	RadixCommitLabel          = "radix-commit"
	RadixImageTagLabel        = "radix-image-tag"
	RadixBranchLabel          = "radix-branch"
	RadixJobTypeLabel         = "radix-job-type"
	RadixJobTypeBuild         = "build"
	RadixAppAliasLabel        = "radix-app-alias"
	RadixIsExternalAliasLabel = "radix-app-external-alias"
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
