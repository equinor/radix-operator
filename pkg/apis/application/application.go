package application

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/util/retry"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var logger *log.Entry

// GranterFunction Handle to granter function for granting access to service account token
type GranterFunction func(kubeutil *kube.Kube, app *v1.RadixRegistration, namespace string, serviceAccount *corev1.ServiceAccount) error

// OperatorDefaultUserGroupEnvironmentVariable If users don't provide ad-group, then it should default to this
const OperatorDefaultUserGroupEnvironmentVariable = "RADIXOPERATOR_DEFAULT_USER_GROUP"

// Application Instance variables
type Application struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *v1.RadixRegistration
}

// NewApplication Constructor
func NewApplication(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	registration *v1.RadixRegistration) (Application, error) {

	return Application{
		kubeclient,
		radixclient,
		kubeutil,
		registration}, nil
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (app Application) OnSync() error {
	// The grantAppAdminAccessToMachineUserToken cannot be a part of out automated tests as it assues the
	// secret for the token is automatically created
	return app.OnSyncWithGranterToMachineUserToken(GrantAppAdminAccessToMachineUserToken)
}

// OnSyncWithGranterToMachineUserToken OnSync where handler is passed in, as the granter function cannot be tested and has to be mocked
func (app Application) OnSyncWithGranterToMachineUserToken(machineUserTokenGranter GranterFunction) error {
	radixRegistration := app.registration
	logger = log.WithFields(log.Fields{"registrationName": radixRegistration.GetName(), "registrationNamespace": radixRegistration.GetNamespace()})

	err := app.createAppNamespace()
	if err != nil {
		logger.Errorf("Failed to create app namespace. %v", err)
		return err
	}

	if app.registration.Spec.MachineUser {
		_, err = app.applyMachineUserServiceAccount(machineUserTokenGranter)
		if err != nil {
			logger.Errorf("Failed to create machine user. %v", err)
			return err
		}
	} else {
		err := app.garbageCollectMachineUserNoLongerInSpec()
		if err != nil {
			logger.Errorf("Failed to perform garbage collection of machine user resources: %v", err)
			return err
		}
	}

	logger.Debugf("App namespace created")

	err = app.createLimitRangeOnAppNamespace(utils.GetAppNamespace(radixRegistration.Name))
	if err != nil {
		logger.Errorf("Failed to create limit range on app namespace. %v", err)
		return err
	}

	logger.Debugf("Limit range on app namespace created")

	err = app.applySecretsForPipelines() // create deploy key in app namespace
	if err != nil {
		logger.Errorf("Failed to apply secrets needed by pipeline. %v", err)
		return err
	}

	logger.Debugf("Applied secrets needed by pipelines")

	pipelineServiceAccount, err := app.applyPipelineServiceAccount()
	if err != nil {
		logger.Errorf("Failed to apply service account needed by pipeline. %v", err)
		return err
	}

	err = app.applyRbacOnConfigToMapRunner()
	if err != nil {
		logger.Errorf("Failed to set access permissions needed to copy radix config to configmap: %v", err)
		return err
	}

	err = app.applyRbacOnScanImageRunner()
	if err != nil {
		logger.Errorf("Failed to set access permissions needed to copy vulnerability scan results to configmap: %v", err)
		return err
	}

	logger.Debugf("Applied service account needed by pipelines")

	err = app.applyRbacOnPipelineRunner(pipelineServiceAccount)
	if err != nil {
		logger.Errorf("Failed to set access permissions needed by pipeline: %v", err)
		return err
	}

	logger.Debugf("Applied access permissions needed by pipeline")

	err = app.applyRbacRadixRegistration()
	if err != nil {
		logger.Errorf("Failed to set access on RadixRegistration: %v", err)
		return err
	}

	logger.Debugf("Applied access permissions to RadixRegistration")

	err = app.grantAccessToCICDLogs()
	if err != nil {
		logger.Errorf("Failed to grant access to ci/cd logs: %v", err)
		return err
	}

	logger.Debugf("Applied access to ci/cd logs. Set registration to be reconciled")
	err = app.updateRadixRegistrationStatus(radixRegistration, func(currStatus *v1.RadixRegistrationStatus) {
		currStatus.Reconciled = metav1.NewTime(time.Now().UTC())
	})
	if err != nil {
		logger.Errorf("Failed to update status on registration: %v", err)
		return err
	}
	return nil
}

func (app *Application) updateRadixRegistrationStatus(rr *v1.RadixRegistration, changeStatusFunc func(currStatus *v1.RadixRegistrationStatus)) error {
	rrInterface := app.radixclient.RadixV1().RadixRegistrations()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentRR, err := rrInterface.Get(context.TODO(), rr.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentRR.Status)
		_, err = rrInterface.UpdateStatus(context.TODO(), currentRR, metav1.UpdateOptions{})

		if err == nil && rr.GetName() == app.registration.GetName() {
			currentRR, err = rrInterface.Get(context.TODO(), rr.GetName(), metav1.GetOptions{})
			if err == nil {
				app.registration = currentRR
			}
		}
		return err
	})
}

// Garbage collect machine user resources
func (app Application) garbageCollectMachineUserNoLongerInSpec() error {
	err := app.garbageCollectMachineUserServiceAccount()
	if err != nil {
		logger.Errorf("Failed to perform garbage collection of service account: %v", err)
		return err
	}
	err = app.removeMachineUserFromPlatformUserRole()
	if err != nil {
		logger.Errorf("Failed to remove machine user from platform user role: %v", err)
		return err
	}
	err = app.removeAppAdminAccessToMachineUserToken()
	if err != nil {
		logger.Errorf("Failed to remove app admin access to machine user: %v", err)
		return err
	}
	return nil
}

// GetAdGroups Gets ad-groups from registration. If missing, gives default for cluster
func GetAdGroups(registration *v1.RadixRegistration) ([]string, error) {
	if registration.Spec.AdGroups == nil || len(registration.Spec.AdGroups) <= 0 {
		defaultGroup := os.Getenv(OperatorDefaultUserGroupEnvironmentVariable)
		if defaultGroup == "" {
			err := fmt.Errorf("Cannot obtain ad-group as %s has not been set for the operator", OperatorDefaultUserGroupEnvironmentVariable)
			logger.Error(err)
			return []string{}, err
		}

		return []string{defaultGroup}, nil
	}

	return registration.Spec.AdGroups, nil
}
