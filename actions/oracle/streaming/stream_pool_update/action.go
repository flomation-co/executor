// Package oracle_streaming_stream_pool_update applies a partial update to a stream pool — its
// name and/or tags. Only the fields you supply are changed; anything left blank is untouched.
// Asynchronous: the pool comes back UPDATING and the response carries a work request OCID.
package oracle_streaming_stream_pool_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Update Stream Pool"
	Description  = "Update a stream pool by OCID — change its name and/or its freeform and defined tags. Only the fields you supply are changed. Returns the pool in an UPDATING state along with a work request OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "stream_pool_ocid", Type: core.ConnectionTypeString, Label: "Stream Pool OCID", Placeholder: "ocid1.streampool.oc1..aaaa…", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Stream Pool Name", Placeholder: "New name (leave blank to keep the current name)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
	{Name: "defined_tags", Type: core.ConnectionTypeString, Label: "Defined Tags (JSON)", Placeholder: "{\"Ops\":{\"env\":\"prod\"}} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stream_pool", Type: core.ConnectionTypeObject, Label: "Stream Pool"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Stream Pool OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := str.RequiredString("stream_pool_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}

	var details streaming.UpdateStreamPoolDetails
	if name := str.OptionalString("name", inputs); name != "" {
		details.Name = &name
	}
	if tags, err := str.FreeformTags("freeform_tags", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}
	if tags, err := str.DefinedTags("defined_tags", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.DefinedTags = tags
	}

	resp, err := client.UpdateStreamPool(str.Context(), streaming.UpdateStreamPoolRequest{StreamPoolId: &id, UpdateStreamPoolDetails: details})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	pool := str.SummariseStreamPool(&resp.StreamPool)
	return str.Result(fmt.Sprintf("Updating stream pool %q — now %s", pool["name"], pool["lifecycle_state"]), map[string]interface{}{
		"stream_pool":     pool,
		"id":              pool["id"],
		"lifecycle_state": pool["lifecycle_state"],
		"work_request_id": str.Str(resp.OpcWorkRequestId),
	}), nil
}
