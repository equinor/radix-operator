package envvars

const (
	// ComponentContainerRegistry The name of the environment variable, injected into Radix components or jobs, containing the name of the container registry
	ComponentContainerRegistry = "RADIX_CONTAINER_REGISTRY"

	// ComponentDNSZone The environment variable on a radix app giving the dns zone. Will be equal to RADIX_COMMON_DNSZONE
	ComponentDNSZone = "RADIX_DNS_ZONE"
)
