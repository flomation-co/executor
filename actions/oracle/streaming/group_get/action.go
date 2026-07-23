// Package oracle_streaming_group_get reads the current state of a consumer group on a stream —
// its partition reservations and the latest committed offset for each partition. A consumer group
// coordinates a set of instances so each partition is consumed by exactly one member; this action
// surfaces which instance holds each partition, up to what offset it has committed, and when that
// reservation expires. DATA PLANE: the stream's messages endpoint is resolved automatically from
// its OCID, so no raw endpoint URL is ever needed.
package oracle_streaming_group_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Get Consumer Group"
	Description  = "Read the current state of a consumer group on a stream — its partition reservations, the instance holding each partition, and the latest committed offset. The stream's endpoint is resolved automatically."
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
	{Name: "stream_ocid", Type: core.ConnectionTypeString, Label: "Stream OCID", Placeholder: "ocid1.stream.oc1..aaaa…", Required: true},
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Consumer Group", Placeholder: "The name of the consumer group", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "group", Type: core.ConnectionTypeObject, Label: "Consumer Group"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	streamID, err := str.RequiredString("stream_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	groupName, err := str.RequiredString("group_name", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := str.DataPlaneClientForStream(inputs, streamID)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.GetGroup(str.Context(), streaming.GetGroupRequest{StreamId: &streamID, GroupName: &groupName})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	reservations := make([]map[string]interface{}, 0, len(resp.Reservations))
	for i := range resp.Reservations {
		r := resp.Reservations[i]
		reservations = append(reservations, map[string]interface{}{
			"partition":           str.Str(r.Partition),
			"committed_offset":    str.Int64OrNil(r.CommittedOffset),
			"reserved_instance":   str.Str(r.ReservedInstance),
			"time_reserved_until": str.FormatTime(r.TimeReservedUntil),
		})
	}
	group := map[string]interface{}{
		"stream_id":    str.Str(resp.StreamId),
		"group_name":   str.Str(resp.GroupName),
		"reservations": reservations,
	}
	return str.Result(fmt.Sprintf("Consumer group %q has %d partition reservation(s)", str.Str(resp.GroupName), len(reservations)), map[string]interface{}{
		"group": group,
	}), nil
}
