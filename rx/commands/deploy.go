package commands

import (
	"github.com/spf13/cobra"
)

const deployUsage = `Manage deploys

Work with Radix Deployments
`

func init(){
	Root.AddCommand(deploy)
}

var deploy = &cobra.Command{
	Use: "deploy",
	Short: "Manage deploys",
	Long: deployUsage,
}