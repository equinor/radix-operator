package internal

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/random"
)

// GetShortName Get short name
func GetShortName(name string) string {
	if len(name) > 4 {
		name = name[:4]
	}
	return fmt.Sprintf("%s-%s", name, strings.ToLower(random.RandStringStrSeed(5, name)))
}
