// Package oracle_streaming_connect_harness_update applies a partial update to a connect harness.
// OCI's UpdateConnectHarness details carry ONLY tags (freeform and defined) — there is no rename
// field on the wire, so this action changes tags and leaves everything else untouched. Only the
// fields you supply are changed. Asynchronous: the harness comes back UPDATING and the response
// carries a work request OCID.
package oracle_streaming_connect_harness_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Update Connect Harness"
	Description  = "Update a connect harness by OCID — change its freeform and defined tags. Only the fields you supply are changed. Returns the harness in an UPDATING state along with a work request OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "connect_harness_ocid", Type: core.ConnectionTypeString, Label: "Connect Harness OCID", Placeholder: "ocid1.connectharness.oc1..aaaa…", Required: true},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
	{Name: "defined_tags", Type: core.ConnectionTypeString, Label: "Defined Tags (JSON)", Placeholder: "{\"Ops\":{\"env\":\"prod\"}} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "connect_harness", Type: core.ConnectionTypeObject, Label: "Connect Harness"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Connect Harness OCID"},
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
	id, err := str.RequiredString("connect_harness_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}

	var details streaming.UpdateConnectHarnessDetails
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

	resp, err := client.UpdateConnectHarness(str.Context(), streaming.UpdateConnectHarnessRequest{ConnectHarnessId: &id, UpdateConnectHarnessDetails: details})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	harness := str.SummariseConnectHarness(&resp.ConnectHarness)
	return str.Result(fmt.Sprintf("Updating connect harness %q — now %s", harness["name"], harness["lifecycle_state"]), map[string]interface{}{
		"connect_harness": harness,
		"id":              harness["id"],
		"lifecycle_state": harness["lifecycle_state"],
		"work_request_id": str.Str(resp.OpcWorkRequestId),
	}), nil
}
