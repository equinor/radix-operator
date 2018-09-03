package onpush

import "fmt"

func getImagePath(appName, componentName, imageTag string) string {
	containerRegistryURL := "radixdev.azurecr.io" // const
	imageName := fmt.Sprintf("%s/%s-%s:%s", containerRegistryURL, appName, componentName, imageTag)
	return imageName
}
