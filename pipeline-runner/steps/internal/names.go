package internal

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
)

// GetShortName Get short name
func GetShortName(name string) string {
	if len(name) > 4 {
		name = name[:4]
	}
	return fmt.Sprintf("%s-%s", name, strings.ToLower(utils.RandStringStrSeed(5, name)))
}
