package kube

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// Radix Annotations
const (
	RadixBranchAnnotation = "radix-branch"

	// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
	RestoredStatusAnnotation = "equinor.com/velero-restored-status"
)

// Radix Labels
const (
	RadixAppLabel                = "radix-app"
	RadixEnvLabel                = "radix-env"
	RadixComponentLabel          = "radix-component"
	RadixJobNameLabel            = "radix-job-name"
	RadixBuildLabel              = "radix-build"
	RadixCommitLabel             = "radix-commit"
	RadixImageTagLabel           = "radix-image-tag"
	RadixJobTypeLabel            = "radix-job-type"
	RadixJobTypeBuild            = "build"
	RadixAppAliasLabel           = "radix-app-alias"
	RadixExternalAliasLabel      = "radix-app-external-alias"
	RadixActiveClusterAliasLabel = "radix-app-active-cluster-alias"

	// Only for backward compatibility
	RadixBranchDeprecated = "radix-branch"
)

// Kube  Stuct for accessing lower level kubernetes functions
type Kube struct {
	kubeClient kubernetes.Interface
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "kube-api"})
}

// New Constructor
func New(client kubernetes.Interface) (*Kube, error) {
	kube := &Kube{
		kubeClient: client,
	}
	return kube, nil
}

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
