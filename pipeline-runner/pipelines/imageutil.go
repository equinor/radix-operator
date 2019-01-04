package onpush

import "fmt"

func getImagePath(infrastructureEnvironment, appName, componentName, imageTag string) string {
	containerRegistryURL := fmt.Sprintf("radix%s.azurecr.io", infrastructureEnvironment)
	imageName := fmt.Sprintf("%s/%s-%s:%s", containerRegistryURL, appName, componentName, imageTag)
	return imageName
}
