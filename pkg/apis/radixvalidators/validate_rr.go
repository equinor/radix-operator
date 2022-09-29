package radixvalidators

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	errorUtils "github.com/equinor/radix-common/utils/errors"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CanRadixRegistrationBeInserted Validates RR
func CanRadixRegistrationBeInserted(client radixclient.Interface, radixRegistration *v1.RadixRegistration) error {
	// cannot be used from admission control - returns the same radix reg that we try to validate
	errUniqueAppName := validateDoesNameAlreadyExist(client, radixRegistration.Name)

	err := CanRadixRegistrationBeUpdated(radixRegistration)
	return errorUtils.Concat([]error{errUniqueAppName, err})
}

// CanRadixRegistrationBeUpdated Validates update of RR
func CanRadixRegistrationBeUpdated(radixRegistration *v1.RadixRegistration) error {
	var errs []error

	if err := validateAppName(radixRegistration.Name); err != nil {
		errs = append(errs, err)
	}

	if err := validateEmail("owner", radixRegistration.Spec.Owner); err != nil {
		errs = append(errs, err)
	}

	if err := validateWbs(radixRegistration.Spec.WBS); err != nil {
		errs = append(errs, err)
	}

	if err := validateGitSSHUrl(radixRegistration.Spec.CloneURL); err != nil {
		errs = append(errs, err)
	}

	if err := validateSSHKey(radixRegistration.Spec.DeployKey); err != nil {
		errs = append(errs, err)
	}

	if err := validateAdGroups(radixRegistration.Spec.AdGroups); err != nil {
		errs = append(errs, err)
	}

	if err := validateConfigBranch(radixRegistration.Spec.ConfigBranch); err != nil {
		errs = append(errs, err)
	}

	return errorUtils.Concat(errs)
}

// GetRadixRegistrationBeInsertedWarnings Get warnings for inserting RadixRegistration
func GetRadixRegistrationBeInsertedWarnings(client radixclient.Interface, radixRegistration *v1.RadixRegistration) ([]string, error) {
	return appendNoDuplicateGitRepoWarning(client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
}

// GetRadixRegistrationBeUpdatedWarnings Get warnings for updating RadixRegistration
func GetRadixRegistrationBeUpdatedWarnings(client radixclient.Interface, radixRegistration *v1.RadixRegistration) ([]string, error) {
	return appendNoDuplicateGitRepoWarning(client, radixRegistration.Name, radixRegistration.Spec.CloneURL)
}

func validateDoesNameAlreadyExist(client radixclient.Interface, appName string) error {
	rr, _ := client.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if rr != nil && rr.Name != "" {
		return fmt.Errorf("app name must be unique in cluster - %s already exist", appName)
	}
	return nil
}

func validateEmail(resourceName, email string) error {
	// re := regexp.MustCompile("(?:[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*|\"(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21\\x23-\\x5b\\x5d-\\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])*\")@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9-]*[a-z0-9]:(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21-\\x5a\\x53-\\x7f]|\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])+)\\])")
	re := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	isValid := re.MatchString(email)
	if isValid {
		return nil
	}
	return InvalidEmailError(resourceName, email)
}

func validateAppName(appName string) error {
	return validateRequiredResourceName("app name", appName)
}

func validateWbs(wbs string) error {
	value := strings.Trim(wbs, " ")
	if len(value) == 0 {
		return ResourceNameCannotBeEmptyError("WBS")
	}
	if len(value) < 5 {
		return InvalidStringValueMinLengthError("WBS", value, 5)
	}
	if len(value) > 100 {
		return InvalidStringValueMaxLengthError("WBS", value, 100)
	}

	re := regexp.MustCompile(`^(([\w\d][\w\d\.]*)?[\w\d])?$`)

	isValid := re.MatchString(wbs)
	if isValid {
		return nil
	}

	return errors.New("WBS can only consist of alphanumeric characters and '.'")
}

func validateRequiredResourceName(resourceName, value string) error {
	if len(value) > 253 {
		return InvalidStringValueMaxLengthError(resourceName, value, 253)
	}

	if value == "" {
		return ResourceNameCannotBeEmptyError(resourceName)
	}

	re := regexp.MustCompile(`^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$`)

	isValid := re.MatchString(value)
	if !isValid {
		return InvalidLowerCaseAlphaNumericDotDashResourceNameError(resourceName, value)
	}

	return nil
}

func validateAdGroups(groups []string) error {
	re := regexp.MustCompile("^([A-Za-z0-9]{8})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{4})-([A-Za-z0-9]{12})$")

	if groups == nil || len(groups) <= 0 {
		// If Ad-group is missing from spec the operator will
		// set a default ad-group provided for the cluster
		return nil
	}

	for _, group := range groups {
		isValid := re.MatchString(group)
		if !isValid {
			return fmt.Errorf("refer ad group %s by object id. It should be in uuid format %s", group, re.String())
		}
	}
	return nil
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

func validateSSHKey(deployKey string) error {
	// todo - how can this be validated..e.g. checked that the key isn't protected by a password
	return nil
}

func validateDoesRRExist(client radixclient.Interface, appName string) error {
	rr, err := client.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if rr == nil || err != nil {
		log.Debugf("error: %v", err)
		return NoRegistrationExistsForApplicationError(appName)
	}
	return nil
}

func validateConfigBranch(name string) error {
	if name == "" {
		return ResourceNameCannotBeEmptyError("branch name")
	}

	if !branch.IsValidName(name) {
		return InvalidConfigBranchName(name)
	}

	return nil
}
