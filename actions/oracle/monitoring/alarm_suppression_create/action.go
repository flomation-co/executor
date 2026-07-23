// Package oracle_monitoring_alarm_suppression_create creates an alarm suppression: a scheduled
// window during which a specific alarm is muted so it does not fire (e.g. a planned maintenance
// outage). This targets the whole alarm at ALARM level — no dimension filter — so it silences the
// alarm regardless of which dimensions breach. Synchronous — returns the created suppression.
package oracle_monitoring_alarm_suppression_create

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
	Name         = "OCI Monitoring: Create Alarm Suppression"
	Description  = "Mute an alarm for a scheduled window (e.g. a planned maintenance outage). The alarm is suppressed at ALARM level between the from/until times so it will not fire. Times are RFC3339."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where the suppression lives)", Required: true},
	{Name: "alarm_ocid", Type: core.ConnectionTypeString, Label: "Alarm OCID", Placeholder: "ocid1.alarm.oc1..aaaa… (the alarm to suppress)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the suppression", Required: true},
	{Name: "time_suppress_from", Type: core.ConnectionTypeString, Label: "Suppress From", Placeholder: "RFC3339 start, e.g. 2026-07-22T01:00:00Z", Required: true},
	{Name: "time_suppress_until", Type: core.ConnectionTypeString, Label: "Suppress Until", Placeholder: "RFC3339 end, e.g. 2026-07-22T03:00:00Z", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Reason for the suppression, e.g. Planned outage IT-1234 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm_suppression", Type: core.ConnectionTypeObject, Label: "Alarm Suppression"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Suppression OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := mon.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	if _, err := auth.RequiredCompartment(); err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	alarmID, err := mon.RequiredString("alarm_ocid", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	name, err := mon.RequiredString("display_name", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	fromRaw, err := mon.RequiredString("time_suppress_from", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	from, perr := time.Parse(time.RFC3339, fromRaw)
	if perr != nil {
		return mon.ErrorResult("suppress from must be RFC3339, e.g. 2026-07-22T01:00:00Z"), nil
	}
	untilRaw, err := mon.RequiredString("time_suppress_until", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	until, perr := time.Parse(time.RFC3339, untilRaw)
	if perr != nil {
		return mon.ErrorResult("suppress until must be RFC3339, e.g. 2026-07-22T03:00:00Z"), nil
	}

	details := monitoring.CreateAlarmSuppressionDetails{
		AlarmSuppressionTarget: monitoring.AlarmSuppressionAlarmTarget{AlarmId: &alarmID},
		DisplayName:            &name,
		TimeSuppressFrom:       &ocicommon.SDKTime{Time: from.UTC()},
		TimeSuppressUntil:      &ocicommon.SDKTime{Time: until.UTC()},
		// Suppress the entire alarm. The API defaults Level to DIMENSION, which then requires a
		// non-empty dimensions filter — we don't collect one, so pin Level to ALARM.
		Level: monitoring.AlarmSuppressionLevelAlarm,
	}
	if v := mon.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.CreateAlarmSuppression(mon.Context(), monitoring.CreateAlarmSuppressionRequest{
		CreateAlarmSuppressionDetails: details,
	})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}
	s := resp.AlarmSuppression
	suppression := map[string]interface{}{
		"id":                  mon.Str(s.Id),
		"display_name":        mon.Str(s.DisplayName),
		"compartment_id":      mon.Str(s.CompartmentId),
		"alarm_id":            alarmID,
		"level":               string(s.Level),
		"lifecycle_state":     string(s.LifecycleState),
		"time_suppress_from":  mon.FormatTime(s.TimeSuppressFrom),
		"time_suppress_until": mon.FormatTime(s.TimeSuppressUntil),
		"time_created":        mon.FormatTime(s.TimeCreated),
		"description":         mon.Str(s.Description),
	}
	return mon.Result(fmt.Sprintf("Created alarm suppression %q (%s)", suppression["display_name"], suppression["lifecycle_state"]), map[string]interface{}{
		"alarm_suppression": suppression, "id": suppression["id"],
	}), nil
}
