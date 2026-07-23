// Package oracle_streaming_group_update forcefully resets a consumer group's position in a stream,
// moving every consumer in the group to a new location at once. Choose LATEST (skip to the tail and
// only read new messages), TRIM_HORIZON (rewind to the oldest retained message) or AT_TIME (jump to
// a specific timestamp — supply the time). The stream's messages endpoint is resolved automatically
// from its OCID; the response carries no body, so the action returns a confirmation.
package oracle_streaming_group_update

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Reset Consumer Group"
	Description  = "Forcefully move a consumer group to a new position in a stream, resetting every consumer at once — to the latest messages, the oldest retained message, or a specific time. The stream's endpoint is resolved automatically."
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
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Consumer Group", Placeholder: "The name of the consumer group to reset", Required: true},
	{Name: "reset_type", Type: core.ConnectionTypeString, Label: "Reset To", Required: true, Options: []core.ConnectionOption{
		{Name: "Latest (only new messages)", Value: "LATEST"},
		{Name: "Trim Horizon (oldest retained message)", Value: "TRIM_HORIZON"},
		{Name: "At Time (a specific timestamp)", Value: "AT_TIME"},
	}},
	{Name: "time", Type: core.ConnectionTypeString, Label: "Time", Placeholder: "RFC3339, e.g. 2026-12-31T00:00:00Z (required when Reset To is At Time)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	resetType, err := str.RequiredString("reset_type", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	if _, ok := streaming.GetMappingUpdateGroupDetailsTypeEnum(resetType); !ok {
		return str.ErrorResult(fmt.Sprintf("reset type %q is not valid — choose LATEST, TRIM_HORIZON or AT_TIME", resetType)), nil
	}

	details := streaming.UpdateGroupDetails{Type: streaming.UpdateGroupDetailsTypeEnum(resetType)}
	// AT_TIME jumps to a timestamp, so a time is mandatory for it; for the other types the service
	// ignores any time, so we simply omit it.
	if v := strings.TrimSpace(str.OptionalString("time", inputs)); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return str.ErrorResult(fmt.Sprintf("invalid time %q: expected RFC3339 (e.g. 2026-12-31T00:00:00Z)", v)), nil
		}
		details.Time = &common.SDKTime{Time: t}
	} else if strings.EqualFold(resetType, "AT_TIME") {
		return str.ErrorResult("time is required when Reset To is At Time (RFC3339, e.g. 2026-12-31T00:00:00Z)"), nil
	}

	auth, client, errResult := str.DataPlaneClientForStream(inputs, streamID)
	if errResult != nil {
		return errResult, nil
	}
	if _, err := client.UpdateGroup(str.Context(), streaming.UpdateGroupRequest{
		StreamId:           &streamID,
		GroupName:          &groupName,
		UpdateGroupDetails: details,
	}); err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}

	return str.Result(fmt.Sprintf("Reset consumer group %q to %s", groupName, resetType), nil), nil
}
