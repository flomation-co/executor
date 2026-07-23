// Package oracle_apigateway_gateway_update applies a partial update to an API Gateway: only the
// display name you supply is changed; a blank field is left unchanged. Asynchronous — the call
// returns a work-request id; poll Get Gateway until the gateway is ACTIVE again.
package oracle_apigateway_gateway_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Update Gateway"
	Description  = "Partially update an API Gateway — change only the display name you supply; a blank field is left unchanged. Returns a work-request id; poll Get Gateway until ACTIVE."
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
	{Name: "gateway_ocid", Type: core.ConnectionTypeString, Label: "Gateway OCID", Placeholder: "ocid1.apigateway.oc1..aaaa… — the gateway to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Gateway OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.GatewayClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	gatewayID, err := agw.RequiredString("gateway_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied. A blank display name is
	// left nil so the gateway keeps its current name.
	details := apigateway.UpdateGatewayDetails{}
	if v := agw.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}

	resp, err := client.UpdateGateway(agw.Context(), apigateway.UpdateGatewayRequest{
		GatewayId:            &gatewayID,
		UpdateGatewayDetails: details,
	})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	return agw.Result(fmt.Sprintf("Updating gateway %s — poll Get Gateway until ACTIVE", gatewayID), map[string]interface{}{
		"id":              gatewayID,
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
