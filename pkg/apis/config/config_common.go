package config

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type CommonConfig struct {
	DNSZone                    string            `json:"dnsZone" required:"true"`
	ClusterName                string            `json:"clusterName" required:"true"`
	ClusterType                string            `json:"clusterType" required:"true"`
	AppAliasBaseURL            string            `json:"appAliasBaseURL" required:"true"`
	ExternalRegistryAuthSecret string            `json:"externalRegistryAuthSecret"`
	OAuth2Proxy                OAuth2ProxyConfig `json:"oauth2Proxy"`
}

type OAuth2ProxyConfig struct {
	ProxyImage    ContainerImage `json:"proxyImage" required:"true"`
	RedisImage    ContainerImage `json:"redisImage" required:"true"`
	ProxyDefaults radixv1.OAuth2 `json:"proxyDefaults"`
}
