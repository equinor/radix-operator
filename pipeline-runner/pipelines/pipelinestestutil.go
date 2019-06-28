package onpush

import (
	"path/filepath"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

func createRadixApplication(testFilePath string) *v1.RadixApplication {
	fileName, _ := filepath.Abs(testFilePath)
	ra, _ := utils.GetRadixApplication(fileName)
	return ra
}
