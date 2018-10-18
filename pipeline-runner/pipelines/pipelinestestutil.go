package onpush

import (
	"path/filepath"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/utils"
)

func createRadixApplication(testFilePath string) *v1.RadixApplication {
	fileName, _ := filepath.Abs(testFilePath)
	ra, _ := utils.GetRadixApplication(fileName)
	return ra
}
