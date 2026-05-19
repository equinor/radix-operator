package models

// ClusterConfiguration holds cluster configuration environment.
// swagger:model ClusterConfiguration
type ClusterConfiguration struct {
	// ClusterEgressIps List of egress IPs for the cluster. Can be used for whitelisting in external services.
	ClusterEgressIps []string `json:"clusterEgressIps"`

	// ClusterOidcIssuers List of OIDC issuers for the cluster. Can be used for configuring OIDC clients and setting up Federated Credentials.
	ClusterOidcIssuers []string `json:"clusterOidcIssuers"`

	// DNSZone The DNS zone configured for the cluster environment
	//
	// example: qa.radix.equinor.com
	DNSZone string `json:"dnsZone"`

	// ClusterName The name of the cluster
	//
	// example: weekly-40
	ClusterName string `json:"clusterName"`
}
