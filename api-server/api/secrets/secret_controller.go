package secrets

import (
	"encoding/json"
	"net/http"

	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
)

const rootPath = "/applications/{appName}"

type secretController struct {
	*models.DefaultController
	tlsValidator tlsvalidation.Validator
}

// NewSecretController Constructor
func NewSecretController(tlsValidator tlsvalidation.Validator) models.Controller {
	return &secretController{
		tlsValidator: tlsValidator,
	}
}

// GetRoutes List the supported routes of this handler
func (c *secretController) GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/{secretName}",
			Method:      "PUT",
			HandlerFunc: c.ChangeComponentSecret,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/secrets/azure/keyvault/{azureKeyVaultName}",
			Method:      "GET",
			HandlerFunc: c.GetAzureKeyVaultSecretVersions,
		},
		models.Route{
			Path:        rootPath + "/environments/{envName}/components/{componentName}/externaldns/{fqdn}/tls",
			Method:      "PUT",
			HandlerFunc: c.UpdateComponentExternalDNSTLS,
		},
	}
	return routes
}

// ChangeComponentSecret Modifies an application environment component secret
func (c *secretController) ChangeComponentSecret(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/components/{componentName}/secrets/{secretName} environment changeComponentSecret
	// ---
	// summary: Update an application environment component secret
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: secret of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: secret component of Radix application
	//   type: string
	//   required: true
	// - name: secretName
	//   in: path
	//   description: environment component secret name to be updated
	//   type: string
	//   required: true
	// - name: componentSecret
	//   in: body
	//   description: New secret value
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/SecretParameters"
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-separated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	secretName := mux.Vars(r)["secretName"]

	var secretParameters secretModels.SecretParameters
	if err := json.NewDecoder(r.Body).Decode(&secretParameters); err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	handler := c.getSecretHandler(accounts)

	if err := handler.ChangeComponentSecret(r.Context(), appName, envName, componentName, secretName, secretParameters); err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

// GetAzureKeyVaultSecretVersions Get Azure Key vault secret versions for a component
func (c *secretController) GetAzureKeyVaultSecretVersions(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /applications/{appName}/environments/{envName}/components/{componentName}/secrets/azure/keyvault/{azureKeyVaultName} environment getAzureKeyVaultSecretVersions
	// ---
	// summary: Get Azure Key vault secret versions for a component
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: secret of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: secret component of Radix application
	//   type: string
	//   required: true
	// - name: azureKeyVaultName
	//   in: path
	//   description: Azure Key vault name
	//   type: string
	//   required: true
	// - name: secretName
	//   in: query
	//   description: secret (or key, cert) name in Azure Key vault
	//   type: string
	//   required: false
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-separated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/AzureKeyVaultSecretVersion"
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	azureKeyVaultName := mux.Vars(r)["azureKeyVaultName"]
	secretName := r.FormValue("secretName")

	handler := c.getSecretHandler(accounts)

	secretStatuses, err := handler.GetAzureKeyVaultSecretVersions(appName, envName, componentName, azureKeyVaultName, secretName)
	if err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, secretStatuses)
}

// UpdateComponentExternalDNSTLS Set external DNS TLS private key and certificate for a component
func (c *secretController) UpdateComponentExternalDNSTLS(accounts models.Accounts, w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /applications/{appName}/environments/{envName}/components/{componentName}/externaldns/{fqdn}/tls component updateComponentExternalDnsTls
	// ---
	// summary: Set external DNS TLS private key certificate for a component
	// parameters:
	// - name: appName
	//   in: path
	//   description: Name of application
	//   type: string
	//   required: true
	// - name: envName
	//   in: path
	//   description: secret of Radix application
	//   type: string
	//   required: true
	// - name: componentName
	//   in: path
	//   description: secret component of Radix application
	//   type: string
	//   required: true
	// - name: fqdn
	//   in: path
	//   description: FQDN to be updated
	//   type: string
	//   required: true
	// - name: tlsData
	//   in: body
	//   description: New TLS private key and certificate
	//   required: true
	//   schema:
	//       "$ref": "#/definitions/UpdateExternalDnsTlsRequest"
	// - name: Impersonate-User
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of test users (Required if Impersonate-Group is set)
	//   type: string
	//   required: false
	// - name: Impersonate-Group
	//   in: header
	//   description: Works only with custom setup of cluster. Allow impersonation of a comma-separated list of test groups (Required if Impersonate-User is set)
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     description: "Invalid application"
	//   "401":
	//     description: "Unauthorized"
	//   "403":
	//     description: "Forbidden"
	//   "404":
	//     description: "Not found"
	//   "409":
	//     description: "Conflict"
	//   "500":
	//     description: "Internal server error"

	appName := mux.Vars(r)["appName"]
	envName := mux.Vars(r)["envName"]
	componentName := mux.Vars(r)["componentName"]
	fqdn := mux.Vars(r)["fqdn"]

	var requestBody secretModels.UpdateExternalDNSTLSRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	handler := c.getSecretHandler(accounts)

	if err := handler.UpdateComponentExternalDNSSecretData(r.Context(), appName, envName, componentName, fqdn, requestBody.Certificate, requestBody.PrivateKey, requestBody.SkipValidation); err != nil {
		c.ErrorResponse(w, r, err)
		return
	}

	c.JSONResponse(w, r, "Success")
}

func (c *secretController) getSecretHandler(accounts models.Accounts) *SecretHandler {
	return Init(WithAccounts(accounts), WithTLSValidator(c.tlsValidator))
}
