package utils

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"os"
	"strings"
)

//GetActiveClusterHostName Gets active cluster name from environment-variable
func GetActiveClusterHostName(componentName, namespace string) string {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s.%s", componentName, namespace, dnsZone)
}

//IsActiveCluster Indicates if cluster name  is for active cluster by environment-variable
func IsActiveCluster(clustername string) bool {
	activeClustername := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	return strings.EqualFold(clustername, activeClustername)
}
