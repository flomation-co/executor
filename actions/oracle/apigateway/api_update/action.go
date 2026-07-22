// Package oracle_apigateway_api_update applies a partial update to an API resource: only the display
// name and specification content you supply are changed; blank fields are left as-is. Asynchronous —
// the call returns a work-request id; poll Get API until the API is ACTIVE before use.
package oracle_apigateway_api_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Update API"
	Description  = "Partially update an API resource — change only the display name or specification content you supply; blank fields are left unchanged. Asynchronous: returns a work-request id — poll Get API until ACTIVE."
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
	{Name: "api_ocid", Type: core.ConnectionTypeString, Label: "API OCID", Placeholder: "ocid1.apigatewayapi.oc1..aaaa… — the API to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Specification Content", Placeholder: "API specification in JSON or YAML (leave blank to keep unchanged)"},
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

	// Partial update: only carry the fields the operator actually supplied so blanks stay unchanged.
	details := apigateway.UpdateApiDetails{}
	if v := agw.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := agw.OptionalString("content", inputs); v != "" {
		details.Content = &v
	}

	resp, err := client.UpdateApi(agw.Context(), apigateway.UpdateApiRequest{ApiId: &apiID, UpdateApiDetails: details})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}
	return agw.Result(fmt.Sprintf("Updating API %s — poll Get API until ACTIVE", apiID), map[string]interface{}{
		"id":              apiID,
		"work_request_id": agw.Str(resp.OpcWorkRequestId),
	}), nil
}
