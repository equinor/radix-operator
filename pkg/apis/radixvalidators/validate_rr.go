package radixvalidators

import (
	"context"
	stderrors "errors"
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	radixConfigFullNamePattern = `^(\/*[a-zA-Z0-9_\.\-]+)+((\.yaml)|(\.yml))$`
)

var (
	ErrInvalidRadixConfigFullName = stderrors.New("invalid file name for radixconfig. See https://www.radix.equinor.com/references/reference-radix-config/ for more information")
	ErrInvalidEntraUuid           = stderrors.New("invalid Entra uuid")
	validUuidRegex                = regexp.MustCompile("^([A-Za-z0-9]{8})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{12})$")

	requiredRadixRegistrationValidators = []RadixRegistrationValidator{
		validateRadixRegistrationAppName,
		validateRadixRegistrationGitSSHUrl,
		validateRadixRegistrationSSHKey,
		validateRadixRegistrationAdConfig,
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
func CanRadixRegistrationBeInserted(ctx context.Context, client radixclient.Interface, radixRegistration *v1.RadixRegistration, additionalValidators ...RadixRegistrationValidator) error {
	// cannot be used from admission control - returns the same radix reg that we try to validate
	validators := append(requiredRadixRegistrationValidators, validateRadixRegistrationAppNameAvailableFactory(ctx, client))
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
	return stderrors.Join(errs...)
}

// GetRadixRegistrationBeInsertedWarnings Get warnings for inserting RadixRegistration
func GetRadixRegistrationBeInsertedWarnings(ctx context.Context, client radixclient.Interface, radixRegistration *v1.RadixRegistration) ([]string, error) {
	return appendNoDuplicateGitRepoWarning(ctx, client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
}

// GetRadixRegistrationBeUpdatedWarnings Get warnings for updating RadixRegistration
func GetRadixRegistrationBeUpdatedWarnings(ctx context.Context, client radixclient.Interface, radixRegistration *v1.RadixRegistration) ([]string, error) {
	return appendNoDuplicateGitRepoWarning(ctx, client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
}

func validateRadixRegistrationAppNameAvailableFactory(ctx context.Context, client radixclient.Interface) RadixRegistrationValidator {
	return func(radixRegistration *v1.RadixRegistration) error {
		return validateDoesNameAlreadyExist(ctx, client, radixRegistration.Name)
	}
}

func validateDoesNameAlreadyExist(ctx context.Context, client radixclient.Interface, appName string) error {
	rr, _ := client.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if rr != nil && rr.Name != "" {
		return fmt.Errorf("app name must be unique in cluster - %s already exist", appName)
	}
	return nil
}

func validateRadixRegistrationAppName(rr *v1.RadixRegistration) error {
	return validateAppName(rr.Name)
}

func validateAppName(appName string) error {
	return validateRequiredResourceName("app name", appName, 253)
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

func validateRequiredResourceName(resourceName, value string, maxLength int) error {
	if len(value) > maxLength {
		return InvalidStringValueMaxLengthErrorWithMessage(resourceName, value, maxLength)
	}

	if value == "" {
		return ResourceNameCannotBeEmptyErrorWithMessage(resourceName)
	}

	re := regexp.MustCompile(resourceNameTemplate)

	isValid := re.MatchString(value)
	if !isValid {
		return InvalidLowerCaseAlphaNumericDashResourceNameErrorWithMessage(resourceName, value)
	}

	return nil
}

func validateRadixRegistrationAdConfig(rr *v1.RadixRegistration) error {
	errs := []error{}
	for _, group := range rr.Spec.AdGroups {
		if !validUuidRegex.MatchString(group) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrInvalidEntraUuid, group))
		}
	}
	for _, group := range rr.Spec.AdUsers {
		if !validUuidRegex.MatchString(group) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrInvalidEntraUuid, group))
		}
	}

	err := stderrors.Join(errs...)
	if err != nil {
		return fmt.Errorf("radix registration Entra validation error: %w", err)
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

func appendNoDuplicateGitRepoWarning(ctx context.Context, client radixclient.Interface, appName, sshURL string) ([]string, error) {
	if sshURL == "" {
		return nil, nil
	}

	registrations, err := client.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
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

func validateDoesRRExist(ctx context.Context, client radixclient.Interface, appName string) error {
	_, err := client.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
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
