// Package oracle_streaming_stream_update applies a partial update to a stream — moving it to a
// different stream pool and/or replacing its tags. Only the fields the operator supplies are sent,
// so unspecified attributes are left untouched. Asynchronous: the stream comes back UPDATING with a
// work request OCID; poll the Get Stream action until it is ACTIVE again.
package oracle_streaming_stream_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Update Stream"
	Description  = "Update a stream in place. Move it to a different stream pool and/or replace its freeform or defined tags — only the fields you provide are changed. Returns the stream in an UPDATING state; poll Get Stream until ACTIVE."
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
	{Name: "stream_ocid", Type: core.ConnectionTypeString, Label: "Stream OCID", Placeholder: "ocid1.stream.oc1..aaaa…", Required: true},
	{Name: "stream_pool_id", Type: core.ConnectionTypeString, Label: "Move to Stream Pool OCID", Placeholder: "ocid1.streampool.oc1..aaaa… (optional — moves the stream)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} — replaces all freeform tags (optional)"},
	{Name: "defined_tags", Type: core.ConnectionTypeString, Label: "Defined Tags (JSON)", Placeholder: "{\"Ops\":{\"env\":\"prod\"}} — replaces all defined tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stream", Type: core.ConnectionTypeObject, Label: "Stream"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Stream OCID"},
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
	id, err := str.RequiredString("stream_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied, mirroring the
	// nil-strip semantics of stream_create. An empty body would be a no-op update.
	details := streaming.UpdateStreamDetails{}
	provided := false
	if poolID := str.OptionalString("stream_pool_id", inputs); poolID != "" {
		details.StreamPoolId = &poolID
		provided = true
	}
	if tags, err := str.FreeformTags("freeform_tags", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
		provided = true
	}
	if tags, err := str.DefinedTags("defined_tags", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.DefinedTags = tags
		provided = true
	}
	if !provided {
		return str.ErrorResult("provide at least one field to update — a stream pool to move to, freeform tags, or defined tags"), nil
	}

	resp, err := client.UpdateStream(str.Context(), streaming.UpdateStreamRequest{
		StreamId:            &id,
		UpdateStreamDetails: details,
	})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	stream := str.SummariseStream(&resp.Stream)
	return str.Result(fmt.Sprintf("Updating stream %q — now %s", stream["name"], stream["lifecycle_state"]), map[string]interface{}{
		"stream":          stream,
		"id":              stream["id"],
		"lifecycle_state": stream["lifecycle_state"],
		"work_request_id": str.Str(resp.OpcWorkRequestId),
	}), nil
}
