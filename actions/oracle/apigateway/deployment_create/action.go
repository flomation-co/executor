// Package oracle_apigateway_deployment_create creates a deployment — an API published on a gateway
// under a path prefix, with its routes defined by an API specification. Asynchronous: the deployment
// comes back with a work-request id; poll the Get Deployment action until it is ACTIVE before use.
package oracle_apigateway_deployment_create

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Create Deployment"
	Description  = "Publish an API on a gateway under a path prefix, with its routes defined by an API specification. Returns a work-request id — poll Get Deployment until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "gateway_ocid", Type: core.ConnectionTypeString, Label: "Gateway OCID", Placeholder: "ocid1.apigateway.oc1..aaaa… — the gateway to deploy on", Required: true},
	{Name: "path_prefix", Type: core.ConnectionTypeString, Label: "Path Prefix", Placeholder: "e.g. /v1 — the path all routes deploy under", Required: true},
	{Name: "specification_json", Type: core.ConnectionTypeText, Label: "API Specification (JSON)", Placeholder: "{\"routes\":[{\"path\":\"/hello\",\"methods\":[\"GET\"],\"backend\":{\"type\":\"STOCK_RESPONSE_BACKEND\",\"status\":200,\"body\":\"Hello\"}}]}", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the deployment (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "deployment", Type: core.ConnectionTypeObject, Label: "Deployment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Endpoint"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.DeploymentClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	gatewayID, err := agw.RequiredString("gateway_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	pathPrefix, err := agw.RequiredString("path_prefix", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	specRaw, err := agw.RequiredString("specification_json", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	var spec apigateway.ApiSpecification
	if err := json.Unmarshal([]byte(specRaw), &spec); err != nil {
		return agw.ErrorResult(fmt.Sprintf("API specification must be a JSON object (e.g. {\"routes\":[…]}): %s", err.Error())), nil
	}

	details := apigateway.CreateDeploymentDetails{
		CompartmentId: &compartment,
		GatewayId:     &gatewayID,
		PathPrefix:    &pathPrefix,
		Specification: &spec,
	}
	if name := strings.TrimSpace(agw.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateDeployment(agw.Context(), apigateway.CreateDeploymentRequest{CreateDeploymentDetails: details})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	// The async create returns the deployment in a CREATING state — surface its OCID so the caller
	// can wire it straight into a Get Deployment poll without a separate list (per Dan's review).
	dep := agw.SummariseDeployment(&resp.Deployment)
	return agw.Result(fmt.Sprintf("Creating deployment on gateway %s under %q — poll Get Deployment until ACTIVE", gatewayID, pathPrefix), map[string]interface{}{
		"deployment":      dep,
		"id":              dep["id"],
		"lifecycle_state": dep["lifecycle_state"],
		"endpoint":        dep["endpoint"],
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
