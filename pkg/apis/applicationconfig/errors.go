package applicationconfig

import "fmt"

// RadixDNSAliasAlreadyUsedByAnotherApplicationError Error when RadixDNSAlias already used by another application
func RadixDNSAliasAlreadyUsedByAnotherApplicationError(domain string) error {
	return fmt.Errorf("DNS alias %s already used by another application", domain)
}
