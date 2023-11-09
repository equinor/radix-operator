package dnsalias

// DNSConfig Config settings for the cluster DNS
type DNSConfig struct {
	// DNSZone Cluster DNS zone.
	// Example radix.equinor.com, playground.radix.equinor.com
	DNSZone string
	// ReservedAppDNSAliases The list of DNS aliases, reserved for Radix platform Radix applications
	ReservedAppDNSAliases map[string]string
	// ReservedDNSAlias The list of DNS aliases, reserved for Radix platform services
	ReservedDNSAlias []string
}

// AppReservedDNSAlias DNS aliases, reserved for Radix application
type AppReservedDNSAlias map[string]string // map[dnsAlias]appName
