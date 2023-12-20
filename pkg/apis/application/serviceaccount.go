package application

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// ApplyPipelineServiceAccount create service account needed by pipeline
func (app *Application) applyPipelineServiceAccount() (*corev1.ServiceAccount, error) {
	return app.kubeutil.CreateServiceAccount(utils.GetAppNamespace(app.registration.Name), defaults.PipelineServiceAccountName)
}
