package radixutils

import (
	"fmt"
	"regexp"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

func IsValidRadixRegistration(registration *radixv1.RadixRegistration) (bool, []error) {
	errs := []error{}
	err := validateGitSSHUrl(registration.Spec.CloneURL)
	if err != nil {
		errs = append(errs, err)
	}
	err = validateSSHKey(registration.Spec.DeployKey)
	if err != nil {
		errs = append(errs, err)
	}
	err = validateAdGroups(registration.Spec.AdGroups)
	if err != nil {
		errs = append(errs, err)
	}

	return len(errs) <= 0, errs
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

func validateGitSSHUrl(sshURL string) error {
	re := regexp.MustCompile("^(git@github.com:)(\\w+)/(\\w.+)(.git)$")

	isValid := re.MatchString(sshURL)

	if isValid {
		return nil
	}
	return fmt.Errorf("ssh url not valid %s. Must match regex %s", sshURL, re.String())
}

func validateSSHKey(deployKey string) error {
	// todo - how can this be validated..e.g. checked that the key isn't protected by a password
	return nil
}
