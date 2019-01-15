package onpush

import "fmt"

func getImagePath(containerRegistry, appName, componentName, imageTag string) string {
	imageName := fmt.Sprintf("%s/%s-%s:%s", containerRegistry, appName, componentName, imageTag)
	return imageName
}
