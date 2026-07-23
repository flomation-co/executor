// Package oracle_generativeai_endpoint_update applies a partial update to a Generative AI endpoint:
// only the display name and description you supply are changed; blank fields are left as-is.
// Asynchronous — the call returns a work-request id; poll Get Endpoint until it is ACTIVE again.
package oracle_generativeai_endpoint_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Update Endpoint"
	Description  = "Partially update a Generative AI endpoint — change only the display name or description you supply; blank fields are left unchanged. Asynchronous: returns a work-request id, poll Get Endpoint until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+robot"
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
	{Name: "endpoint_ocid", Type: core.ConnectionTypeString, Label: "Endpoint OCID", Placeholder: "ocid1.generativeaiendpoint.oc1..aaaa… — the endpoint to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoint", Type: core.ConnectionTypeObject, Label: "Endpoint"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Endpoint OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := gai.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	endpointID, err := gai.RequiredString("endpoint_ocid", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied; blanks stay unchanged.
	details := generativeai.UpdateEndpointDetails{}
	if v := gai.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := gai.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateEndpoint(gai.Context(), generativeai.UpdateEndpointRequest{
		EndpointId:            &endpointID,
		UpdateEndpointDetails: details,
	})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}

	endpoint := gai.SummariseEndpoint(&resp.Endpoint)
	return gai.Result(fmt.Sprintf("Updating endpoint %q — poll Get Endpoint until ACTIVE", endpoint["display_name"]), map[string]interface{}{
		"endpoint":        endpoint,
		"id":              endpoint["id"],
		"work_request_id": gai.Str(resp.OpcWorkRequestId),
	}), nil
}
