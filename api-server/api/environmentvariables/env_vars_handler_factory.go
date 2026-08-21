package environmentvariables

import "github.com/equinor/radix-operator/api-server/internal/accounts"

type envVarsHandlerFactory interface {
	createHandler(accounts.Accounts) EnvVarsHandler
}

type defaultEnvVarsHandlerFactory struct{}

func (factory *defaultEnvVarsHandlerFactory) createHandler(accounts accounts.Accounts) EnvVarsHandler {
	return Init(WithAccounts(accounts))
}
