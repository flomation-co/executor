// Package oracle_identity_api_key_list lists the API signing keys uploaded for an IAM user.
package oracle_identity_api_key_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List API Keys"
	Description  = "List the API signing keys uploaded for an Oracle Cloud IAM user — each key's fingerprint, OCID and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the user picker)"},
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID (to read)", Placeholder: "ocid1.user.oc1..aaaa… whose API keys to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "api_keys", Type: core.ConnectionTypeObject, Label: "API Keys"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, userID, errResult := iam.ResourceClient(inputs, "target_user_ocid")
	if errResult != nil {
		return errResult, nil
	}
	// ListApiKeys takes a UserId PATH and has no page request parameter, so there is no
	// pagination walk — OpcNextPage on the response only signals a partial list.
	resp, err := client.ListApiKeys(iam.Context(), identity.ListApiKeysRequest{UserId: &userID})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	var out []map[string]interface{}
	for i := range resp.Items {
		out = append(out, iam.SummariseApiKey(&resp.Items[i]))
	}
	truncated := resp.OpcNextPage != nil && *resp.OpcNextPage != ""
	return iam.Result(fmt.Sprintf("Found %d API key(s)", len(out)), map[string]interface{}{
		"api_keys": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
