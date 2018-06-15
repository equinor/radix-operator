package commands

import (
	"errors"
	"io"

	log "github.com/Sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const deployCreateUsage = `Creates a deployment named DEPLOYMENT
`

var (
	app        string
	components map[string]string
)

func init() {
	deployCreate.Flags().StringVarP(&app, "app", "a", "", "The application name")

	deploy.AddCommand(deployCreate)
}

var deployCreate = &cobra.Command{
	Use:   "create DEPLOYMENT",
	Short: "creates a deployment",
	Long:  deployCreateUsage,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("deployment name is a required argument")
		}
		return createDeployment(cmd.OutOrStdout(), args[0])
	},
}

func createDeployment(out io.Writer, name string) error {
	deployment := &v1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.RadixDeploymentSpec{
			Image: "test",
		},
	}

	c, err := radixClient()
	if err != nil {
		return err
	}

	_, err = c.RadixV1().RadixDeployments("default").Create(deployment)
	if err != nil {
		log.Errorf("Failed to create deployment: %v", err)
		return err
	}
	return nil
}
