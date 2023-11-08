package config

// DNSConfig Config settings for the cluster DNS
type DNSConfig struct {
	// DNSZone Cluster DNS zone.
	// Example radix.equinor.com, playground.radix.equinor.com
	DNSZone string
	// DNSAliasAppReserved The list of DNS aliases, reserved for Radix platform Radix applications
	DNSAliasAppReserved map[string]string
	// DNSAliasReserved The list of DNS aliases, reserved for Radix platform services
	DNSAliasReserved []string
}
