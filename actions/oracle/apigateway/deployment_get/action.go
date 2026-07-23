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
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
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
