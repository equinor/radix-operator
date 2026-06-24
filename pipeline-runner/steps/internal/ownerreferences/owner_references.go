package ownerreferences

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// OwnerReferenceFactory The factory to create an owner reference for a pipeline job
type OwnerReferenceFactory interface {
	Create() *metav1.OwnerReference
}

type ownerReferenceFactory struct{}

// NewOwnerReferenceFactory New instance of the factory
func NewOwnerReferenceFactory() OwnerReferenceFactory {
	return &ownerReferenceFactory{}
}

// Create an OwnerReference from the DownwardAPI created file, if exist
func (f *ownerReferenceFactory) Create() *metav1.OwnerReference {
	controllerUid, jobName, err := getOwnerReferencePropertiesFromDownwardsApiFile()
	if err != nil {
		log.Debug().Msgf("%v", err)
		return nil
	}
	return &metav1.OwnerReference{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       jobName,
		UID:        types.UID(controllerUid),
	}
}

func getOwnerReferencePropertiesFromDownwardsApiFile() (string, string, error) {
	labelsFile := "/pod-labels/labels"
	log.Debug().Msgf("read pod-labels from the file %s", labelsFile)
	if _, err := os.Stat(labelsFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("missing the file with labels %s - ownerReference is not set to the pod", labelsFile)
	}
	file, err := os.Open(labelsFile)
	defer func() {
		if err == nil {
			_ = file.Close()
		}
	}()
	if err != nil {
		return "", "", fmt.Errorf("failed to read the labels file %s: %w", labelsFile, err)
	}

	labelsMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		items := strings.Split(scanner.Text(), "=")
		if len(items) == 2 {
			labelsMap[items[0]] = strings.ReplaceAll(items[1], "\"", "")
		}
	}
	controllerUid, ok := labelsMap["controller-uid"]
	if !ok || len(controllerUid) == 0 {
		return "", "", fmt.Errorf("missing the 'controller-uid' label in the file with labels %s - ownerReference is not set to the pod", labelsFile)
	}
	jobName, ok := labelsMap["job-name"]
	if !ok {
		return "", "", fmt.Errorf("missing the 'job-name' label in the file with labels %s - ownerReference is not set to the pod", labelsFile)
	}
	return controllerUid, jobName, nil
}
