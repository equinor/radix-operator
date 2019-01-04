package onpush

import "fmt"

func getImagePath(environment, appName, componentName, imageTag string) string {
	containerRegistryURL := fmt.Sprintf("radix%s.azurecr.io", environment)
	imageName := fmt.Sprintf("%s/%s-%s:%s", containerRegistryURL, appName, componentName, imageTag)
	return imageName
}
