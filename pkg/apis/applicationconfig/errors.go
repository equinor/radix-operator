package applicationconfig

import "fmt"

// RadixDNSAliasAlreadyUsedByAnotherApplicationError Error when RadixDNSAlias already used by another application
func RadixDNSAliasAlreadyUsedByAnotherApplicationError(alias string) error {
	return fmt.Errorf("DNS alias %s already used by another application", alias)
}

// RadixDNSAliasIsReservedForRadixPlatformApplicationError Error when RadixDNSAlias is reserved by Radix platform for a Radix application
func RadixDNSAliasIsReservedForRadixPlatformApplicationError(alias string) error {
	return fmt.Errorf("DNS alias %s is reserved by Radix platform application", alias)
}

// RadixDNSAliasIsReservedForRadixPlatformServiceError Error when RadixDNSAlias is reserved by Radix platform for a Radix service
func RadixDNSAliasIsReservedForRadixPlatformServiceError(alias string) error {
	return fmt.Errorf("DNS alias %s is reserved by Radix platform service", alias)
}
