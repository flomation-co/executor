// Package oracle_monitoring_alarm_history_get retrieves the history entries for an alarm: the
// record of state and rule changes (e.g. OK → Firing) over time, optionally narrowed to a type
// and a timestamp window. Synchronous — walks pagination up to a safe cap.
package oracle_monitoring_alarm_history_get

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	ocicommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Get Alarm History"
	Description  = "Retrieve the history entries for an alarm — the record of state and rule changes (e.g. OK to Firing) over time. Optionally filter by entry type and an RFC3339 timestamp window. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (not sent, but required by the credential block)", Required: true},
	{Name: "alarm_ocid", Type: core.ConnectionTypeString, Label: "Alarm OCID", Placeholder: "ocid1.alarm.oc1..aaaa… (the alarm to read history for)", Required: true},
	{Name: "alarm_history_type", Type: core.ConnectionTypeString, Label: "History Type", Placeholder: "Which entries to retrieve (default: all types)", Options: []core.ConnectionOption{
		{Name: "All types", Value: ""},
		{Name: "State history", Value: "STATE_HISTORY"},
		{Name: "State transition history", Value: "STATE_TRANSITION_HISTORY"},
		{Name: "Rule history", Value: "RULE_HISTORY"},
		{Name: "Rule transition history", Value: "RULE_TRANSITION_HISTORY"},
	}},
	{Name: "timestamp_greater_than_or_equal_to", Type: core.ConnectionTypeString, Label: "From Timestamp", Placeholder: "RFC3339, e.g. 2026-07-01T00:00:00Z — entries on or after this (optional)"},
	{Name: "timestamp_less_than", Type: core.ConnectionTypeString, Label: "To Timestamp", Placeholder: "RFC3339, e.g. 2026-07-22T00:00:00Z — entries before this (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "entries", Type: core.ConnectionTypeObject, Label: "History Entries"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Alarm Enabled"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := mon.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	alarmID, err := mon.RequiredString("alarm_ocid", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}

	req := monitoring.GetAlarmHistoryRequest{AlarmId: &alarmID}
	if v := mon.OptionalString("alarm_history_type", inputs); v != "" {
		req.AlarmHistorytype = monitoring.GetAlarmHistoryAlarmHistorytypeEnum(v)
	}
	if v := mon.OptionalString("timestamp_greater_than_or_equal_to", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return mon.ErrorResult("from timestamp must be RFC3339, e.g. 2026-07-01T00:00:00Z"), nil
		}
		req.TimestampGreaterThanOrEqualTo = &ocicommon.SDKTime{Time: parsed.UTC()}
	}
	if v := mon.OptionalString("timestamp_less_than", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return mon.ErrorResult("to timestamp must be RFC3339, e.g. 2026-07-22T00:00:00Z"), nil
		}
		req.TimestampLessThan = &ocicommon.SDKTime{Time: parsed.UTC()}
	}

	var out []map[string]interface{}
	var isEnabled interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= mon.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.GetAlarmHistory(mon.Context(), req)
		if err != nil {
			return mon.ErrorResult(auth.OCIError(err)), nil
		}
		isEnabled = mon.Bool(resp.IsEnabled)
		for i := range resp.Entries {
			e := &resp.Entries[i]
			out = append(out, map[string]interface{}{
				"alarm_summary":       mon.Str(e.AlarmSummary),
				"summary":             mon.Str(e.Summary),
				"timestamp":           mon.FormatTime(e.Timestamp),
				"timestamp_triggered": mon.FormatTime(e.TimestampTriggered),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return mon.Result(fmt.Sprintf("Found %d alarm history entries", len(out)), map[string]interface{}{
		"entries": out, "count": fmt.Sprintf("%d", len(out)), "is_enabled": isEnabled, "truncated": truncated,
	}), nil
}
