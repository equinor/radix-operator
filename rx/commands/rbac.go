package commands

import (
	"github.com/spf13/cobra"
)

func init() {
	Root.AddCommand(rbac)
}

var rbac = &cobra.Command{
	Use:   "rbac",
	Short: "Manage rbac",
	Long: `Manage rbac

	Work with Rbac on Radix objects
	`,
}
