package test

import "fmt"

// AppNotFoundErrorMsg When app registration is not found
func AppNotFoundErrorMsg(name string) string {
	return fmt.Sprintf("Error: radixregistrations.radix.equinor.com \"%s\" not found", name)
}
