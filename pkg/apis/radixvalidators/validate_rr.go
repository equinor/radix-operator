package radixvalidators

import (
	"context"
	errors2 "errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/equinor/radix-common/utils/errors"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	radixConfigFullNamePattern = `^(\/*[a-zA-Z0-9_\.\-]+)+((\.yaml)|(\.yml))$`
)

var (
	ErrInvalidRadixConfigFullName = errors2.New("invalid file name for radixconfig. See https://www.radix.equinor.com/references/reference-radix-config/ for more information")

	requiredRadixRegistrationValidators []RadixRegistrationValidator = []RadixRegistrationValidator{
		validateRadixRegistrationAppName,
		validateRadixRegistrationGitSSHUrl,
		validateRadixRegistrationSSHKey,
		validateRadixRegistrationAdGroups,
		validateRadixRegistrationConfigBranch,
		validateRadixRegistrationConfigurationItem,
	}
)

// RadixRegistrationValidator defines a validator function for a RadixRegistration
type RadixRegistrationValidator func(radixRegistration *v1.RadixRegistration) error

// RequireConfigurationItem validates that ConfigurationItem for a RadixRegistration set
func RequireConfigurationItem(rr *v1.RadixRegistration) error {
	if len(strings.TrimSpace(rr.Spec.ConfigurationItem)) == 0 {
		return ResourceNameCannotBeEmptyErrorWithMessage("configuration item")
	}

	return nil
}

// RequireAdGroups validates that AdGroups contains minimum one item
func RequireAdGroups(rr *v1.RadixRegistration) error {
	if len(rr.Spec.AdGroups) == 0 {
		return ResourceNameCannotBeEmptyErrorWithMessage("AD groups")
	}

	return nil
}

// CanRadixRegistrationBeInserted Validates RR
func CanRadixRegistrationBeInserted(client radixclient.Interface, radixRegistration *v1.RadixRegistration, additionalValidators ...RadixRegistrationValidator) error {
	// cannot be used from admission control - returns the same radix reg that we try to validate
	validators := append(requiredRadixRegistrationValidators, validateRadixRegistrationAppNameAvailableFactory(client))
	validators = append(validators, additionalValidators...)
	return validateRadixRegistration(radixRegistration, validators...)
}

// CanRadixRegistrationBeUpdated Validates update of RR
func CanRadixRegistrationBeUpdated(radixRegistration *v1.RadixRegistration, additionalValidators ...RadixRegistrationValidator) error {
	validators := append(requiredRadixRegistrationValidators, additionalValidators...)
	return validateRadixRegistration(radixRegistration, validators...)
}

func validateRadixRegistration(radixRegistration *v1.RadixRegistration, validators ...RadixRegistrationValidator) error {
	var errs []error
	for _, v := range validators {
		if err := v(radixRegistration); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}

// GetRadixRegistrationBeInsertedWarnings Get warnings for inserting RadixRegistration
func GetRadixRegistrationBeInsertedWarnings(client radixclient.Interface, radixRegistration *v1.RadixRegistration) ([]string, error) {
	return appendNoDuplicateGitRepoWarning(client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
}

// GetRadixRegistrationBeUpdatedWarnings Get warnings for updating RadixRegistration
func GetRadixRegistrationBeUpdatedWarnings(client radixclient.Interface, radixRegistration *v1.RadixRegistration) ([]string, error) {
	return appendNoDuplicateGitRepoWarning(client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
}

func validateRadixRegistrationAppNameAvailableFactory(client radixclient.Interface) RadixRegistrationValidator {
	return func(radixRegistration *v1.RadixRegistration) error {
		return validateDoesNameAlreadyExist(client, radixRegistration.Name)
	}
}

func validateDoesNameAlreadyExist(client radixclient.Interface, appName string) error {
	rr, _ := client.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if rr != nil && rr.Name != "" {
		return fmt.Errorf("app name must be unique in cluster - %s already exist", appName)
	}
	return nil
}

func validateRadixRegistrationAppName(rr *v1.RadixRegistration) error {
	return validateAppName(rr.Name)
}

func validateAppName(appName string) error {
	return validateRequiredResourceName("app name", appName)
}

func validateRadixRegistrationConfigurationItem(rr *v1.RadixRegistration) error {
	return validateConfigurationItem(rr.Spec.ConfigurationItem)
}

func validateConfigurationItem(value string) error {
	if len(value) > 100 {
		return InvalidStringValueMaxLengthErrorWithMessage("configuration item", value, 100)
	}
	return nil
}

func validateRequiredResourceName(resourceName, value string) error {
	if len(value) > 253 {
		return InvalidStringValueMaxLengthErrorWithMessage(resourceName, value, 253)
	}

	if value == "" {
		return ResourceNameCannotBeEmptyErrorWithMessage(resourceName)
	}

	re := regexp.MustCompile(`^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$`)

	isValid := re.MatchString(value)
	if !isValid {
		return InvalidLowerCaseAlphaNumericDotDashResourceNameErrorWithMessage(resourceName, value)
	}

	return nil
}

func validateRadixRegistrationAdGroups(rr *v1.RadixRegistration) error {
	return validateAdGroups(rr.Spec.AdGroups)
}

func validateAdGroups(groups []string) error {
	re := regexp.MustCompile("^([A-Za-z0-9]{8})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{12})$")
	for _, group := range groups {
		isValid := re.MatchString(group)
		if !isValid {
			return fmt.Errorf("refer ad group %s by object id. It should be in uuid format %s", group, re.String())
		}
	}
	return nil
}

func validateRadixRegistrationGitSSHUrl(rr *v1.RadixRegistration) error {
	return validateGitSSHUrl(rr.Spec.CloneURL)
}

func validateGitSSHUrl(sshURL string) error {
	re := regexp.MustCompile(`^(git@github.com:)([\w-]+)/([\w-]+)(.git)$`)

	if sshURL == "" {
		return fmt.Errorf("ssh url is required")
	}

	isValid := re.MatchString(sshURL)

	if isValid {
		return nil
	}
	return fmt.Errorf("ssh url not valid %s. Must match regex %s", sshURL, re.String())
}

func appendNoDuplicateGitRepoWarning(client radixclient.Interface, appName, sshURL string) ([]string, error) {
	if sshURL == "" {
		return nil, nil
	}

	registrations, err := client.RadixV1().RadixRegistrations().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, reg := range registrations.Items {
		if reg.Spec.CloneURL == sshURL && !strings.EqualFold(reg.Name, appName) {
			return []string{"Repository is used in other application(s)"}, err
		}
	}
	return nil, nil
}

func validateRadixRegistrationSSHKey(rr *v1.RadixRegistration) error {
	return validateSSHKey(rr.Spec.DeployKey)
}

func validateSSHKey(deployKey string) error {
	// todo - how can this be validated..e.g. checked that the key isn't protected by a password
	return nil
}

func validateDoesRRExist(client radixclient.Interface, appName string) error {
	_, err := client.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return NoRegistrationExistsForApplicationErrorWithMessage(appName)
		}
		return err
	}
	return nil
}

func validateRadixRegistrationConfigBranch(rr *v1.RadixRegistration) error {
	return validateConfigBranch(rr.Spec.ConfigBranch)
}

func validateConfigBranch(name string) error {
	if name == "" {
		return ResourceNameCannotBeEmptyErrorWithMessage("branch name")
	}

	if !branch.IsValidName(name) {
		return InvalidConfigBranchNameWithMessage(name)
	}

	return nil
}

// ValidateRadixConfigFullName Validates the radixconfig file name and path
func ValidateRadixConfigFullName(radixConfigFullName string) error {
	if len(radixConfigFullName) == 0 {
		return nil // for empty radixConfigFullName it is used default radixconfig.yaml file name
	}
	matched, err := regexp.Match(radixConfigFullNamePattern, []byte(radixConfigFullName))
	if err != nil {
		return err
	}
	if !matched {
		return ErrInvalidRadixConfigFullName
	}
	return nil
}
