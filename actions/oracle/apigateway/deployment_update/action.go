// Package oracle_apigateway_deployment_update applies a partial update to an API Gateway deployment:
// only the display name and/or the API specification you supply are changed; blank fields are left
// unchanged. Asynchronous — the call returns a work-request id; poll Get Deployment until the
// deployment is ACTIVE again.
package oracle_apigateway_deployment_update

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
	Name         = "OCI API Gateway: Update Deployment"
	Description  = "Partially update an API Gateway deployment — change only the display name and/or API specification (JSON) you supply; blank fields are left unchanged. Returns a work-request id; poll Get Deployment until ACTIVE."
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
	{Name: "deployment_ocid", Type: core.ConnectionTypeString, Label: "Deployment OCID", Placeholder: "ocid1.apideployment.oc1..aaaa… — the deployment to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "specification_json", Type: core.ConnectionTypeText, Label: "API Specification (JSON)", Placeholder: "{\"routes\":[…]} — the deployment specification (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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

	// Partial update: only carry the fields the operator actually supplied. A blank display name is
	// left nil, and a blank specification leaves the deployment's current routes/policies untouched.
	details := apigateway.UpdateDeploymentDetails{}
	if v := agw.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if raw := strings.TrimSpace(agw.OptionalString("specification_json", inputs)); raw != "" {
		var spec apigateway.ApiSpecification
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return agw.ErrorResult(fmt.Sprintf("specification JSON is not valid: %s", err.Error())), nil
		}
		details.Specification = &spec
	}

	resp, err := client.UpdateDeployment(agw.Context(), apigateway.UpdateDeploymentRequest{
		DeploymentId:            &deploymentID,
		UpdateDeploymentDetails: details,
	})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	return agw.Result(fmt.Sprintf("Updating deployment %s — poll Get Deployment until ACTIVE", deploymentID), map[string]interface{}{
		"id":              deploymentID,
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
