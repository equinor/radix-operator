package environmentvariables

import "github.com/equinor/radix-operator/api-server/models"

type envVarsHandlerFactory interface {
	createHandler(models.Accounts) EnvVarsHandler
}

type defaultEnvVarsHandlerFactory struct{}

func (factory *defaultEnvVarsHandlerFactory) createHandler(accounts models.Accounts) EnvVarsHandler {
	return Init(WithAccounts(accounts))
}
