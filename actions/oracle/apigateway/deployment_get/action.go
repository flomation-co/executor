// Package oracle_apigateway_deployment_get fetches a single API Gateway deployment by its OCID,
// returning its gateway, path prefix, public endpoint and lifecycle state.
package oracle_apigateway_deployment_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Get Deployment"
	Description  = "Fetch a single API Gateway deployment by its OCID — its gateway, path prefix, endpoint and lifecycle state."
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
	{Name: "deployment_ocid", Type: core.ConnectionTypeString, Label: "Deployment OCID", Placeholder: "ocid1.apideployment.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "deployment", Type: core.ConnectionTypeObject, Label: "Deployment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Endpoint"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.DeploymentClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	deploymentID, err := agw.RequiredString("deployment_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetDeployment(agw.Context(), apigateway.GetDeploymentRequest{DeploymentId: &deploymentID})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	deployment := agw.SummariseDeployment(&resp.Deployment)
	return agw.Result(fmt.Sprintf("Deployment %q (%s)", deployment["display_name"], deployment["lifecycle_state"]), map[string]interface{}{
		"deployment":      deployment,
		"id":              deployment["id"],
		"lifecycle_state": deployment["lifecycle_state"],
		"endpoint":        deployment["endpoint"],
	}), nil
}
