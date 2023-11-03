package applicationconfig

import "fmt"

// RadixDNSAliasAlreadyUsedByAnotherApplicationError Error when RadixDNSAlias already used by another application
func RadixDNSAliasAlreadyUsedByAnotherApplicationError(domain string) error {
	return fmt.Errorf("DNS alias %s already used by another application", domain)
}

// RadixDNSAliasIsReservedForRadixPlatformApplicationError Error when RadixDNSAlias is reserved by Radix platform for a Radix application
func RadixDNSAliasIsReservedForRadixPlatformApplicationError(domain string) error {
	return fmt.Errorf("DNS alias %s is reserved by Radix platform application", domain)
}

// RadixDNSAliasIsReservedForRadixPlatformServiceError Error when RadixDNSAlias is reserved by Radix platform for a Radix service
func RadixDNSAliasIsReservedForRadixPlatformServiceError(domain string) error {
	return fmt.Errorf("DNS alias %s is reserved by Radix platform service", domain)
}
