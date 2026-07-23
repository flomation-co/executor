// Package oracle_apigateway_api_create creates an API resource — the container that holds a
// versioned API specification (an OpenAPI description) to be published by a deployment. Asynchronous:
// the API comes back in a CREATING state with a work-request id; poll the Get API action (or the work
// request) until it is ACTIVE before deploying it.
package oracle_apigateway_api_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Create API"
	Description  = "Create an API resource holding an OpenAPI specification. Returns the API in a CREATING state plus a work-request id — poll Get API until ACTIVE."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the API (optional)"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Specification Content", Placeholder: "The API specification (OpenAPI) as raw JSON or YAML (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "api", Type: core.ConnectionTypeObject, Label: "API"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "API OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.ApiClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	details := apigateway.CreateApiDetails{CompartmentId: &compartment}
	if name := agw.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if content := agw.OptionalString("content", inputs); content != "" {
		details.Content = &content
	}

	resp, err := client.CreateApi(agw.Context(), apigateway.CreateApiRequest{CreateApiDetails: details})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}

	summary := agw.SummariseApi(&resp.Api)
	return agw.Result(fmt.Sprintf("Creating API %q — poll Get API until ACTIVE", agw.Str(resp.Api.DisplayName)), map[string]interface{}{
		"api":             summary,
		"id":              summary["id"],
		"lifecycle_state": summary["lifecycle_state"],
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
