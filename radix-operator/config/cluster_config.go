package config

// ClusterConfig Config settings for the cluster
type ClusterConfig struct {
	// DNSZone Cluster DNS zone.
	// Example radix.equinor.com, playground.radix.equinor.com
	DNSZone string
}
