// Package oracle_apigateway_api_delete deletes an API resource by its OCID — the container for the
// versioned API specifications served on a gateway. Asynchronous: it returns a work-request id that
// tracks the delete to completion.
package oracle_apigateway_api_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Delete API"
	Description  = "Delete an API resource by its OCID — returns a work-request id that tracks the delete to completion."
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
	{Name: "api_ocid", Type: core.ConnectionTypeString, Label: "API OCID", Placeholder: "ocid1.apigatewayapi.oc1..aaaa… of the API to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "API OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.ApiClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	apiID, err := agw.RequiredString("api_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	resp, err := client.DeleteApi(agw.Context(), apigateway.DeleteApiRequest{ApiId: &apiID})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	return agw.Result(fmt.Sprintf("Deleting API %s", apiID), map[string]interface{}{
		"id":              apiID,
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
