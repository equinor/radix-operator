package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const deployCreateUsage = `Creates a deployment named DEPLOYMENT`
const retrySecond = 30
const maxRetry = 120

var (
	app          string
	image        []string
	environments []string
)

type ComponentDeploy struct {
	Name  string
	Image string
}

func init() {
	defaultEnv := []string{
		"dev",
		"prod",
	}

	deployCreate.Flags().StringVarP(&app, "app", "a", "", "The application name")
	deployCreate.Flags().StringArrayVarP(&image, "image", "i", nil, "Docker image to be deployed (component=image)")
	deployCreate.Flags().StringArrayVarP(&environments, "environments", "e", defaultEnv, "Environments to deploy to (dev,qa,prod)")
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
	components, err := getComponents()
	if err != nil {
		return err
	}
	for _, env := range environments {
		deploys := []v1.RadixDeployComponent{}
		for _, component := range components {
			deploy := v1.RadixDeployComponent{
				Name:  component.Name,
				Image: component.Image,
			}
			deploys = append(deploys, deploy)
		}

		deployment := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1.RadixDeploymentSpec{
				AppName:    app,
				Components: deploys,
			},
		}

		c, err := radixClient()
		if err != nil {
			return err
		}

		created := false
		retry := 0
		for !created {
			log.Infof("Creating RadixDeployment object %s in environment %s", name, env)
			_, err = c.RadixV1().RadixDeployments(fmt.Sprintf("%s-%s", app, env)).Create(deployment)
			if err != nil {
				log.Errorf("Failed to create deployment: %v", err)
				if retry >= maxRetry {
					log.Info("No more retry")
					return err
				}
				log.Info("Retrying in 30 seconds")
				log.Infof("Retry %d of %d", retry, maxRetry)
				retry++
				time.Sleep(time.Duration(retrySecond) * time.Second)
			} else {
				log.Infof("Succeed creating RadixDeployment object %s in environment %s", name, env)
				created = true
			}
		}
	}
	return nil
}

func getComponents() ([]ComponentDeploy, error) {
	components := []ComponentDeploy{}
	if image == nil {
		return nil, errors.New("no image(s) specified")
	}
	for _, c := range image {
		imageData := strings.Split(c, "=")
		component := ComponentDeploy{
			Name:  imageData[0],
			Image: imageData[1],
		}
		components = append(components, component)
	}
	return components, nil
}
