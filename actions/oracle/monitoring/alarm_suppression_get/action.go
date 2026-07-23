// Package oracle_monitoring_alarm_suppression_get fetches a single alarm suppression by OCID: the
// window (and any recurrence preconditions) during which a matching alarm's notifications are
// muted. Synchronous — returns the suppression.
package oracle_monitoring_alarm_suppression_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Get Alarm Suppression"
	Description  = "Fetch a single alarm suppression by its OCID, including the suppression window, target, level, and any recurrence preconditions."
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
	{Name: "alarm_suppression_ocid", Type: core.ConnectionTypeString, Label: "Alarm Suppression OCID", Placeholder: "ocid1.alarmsuppression.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm_suppression", Type: core.ConnectionTypeObject, Label: "Alarm Suppression"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Alarm Suppression OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := mon.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := mon.RequiredString("alarm_suppression_ocid", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetAlarmSuppression(mon.Context(), monitoring.GetAlarmSuppressionRequest{AlarmSuppressionId: &id})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}

	s := resp.AlarmSuppression
	suppression := map[string]interface{}{
		"id":                       mon.Str(s.Id),
		"compartment_id":           mon.Str(s.CompartmentId),
		"display_name":             mon.Str(s.DisplayName),
		"level":                    string(s.Level),
		"alarm_suppression_target": s.AlarmSuppressionTarget,
		"time_suppress_from":       mon.FormatTime(s.TimeSuppressFrom),
		"time_suppress_until":      mon.FormatTime(s.TimeSuppressUntil),
		"lifecycle_state":          string(s.LifecycleState),
		"time_created":             mon.FormatTime(s.TimeCreated),
		"time_updated":             mon.FormatTime(s.TimeUpdated),
		"suppression_conditions":   s.SuppressionConditions,
		"description":              mon.Str(s.Description),
		"dimensions":               s.Dimensions,
		"freeform_tags":            s.FreeformTags,
	}
	return mon.Result(fmt.Sprintf("Fetched alarm suppression %q (%s)", suppression["display_name"], suppression["lifecycle_state"]), map[string]interface{}{
		"alarm_suppression": suppression, "id": suppression["id"],
	}), nil
}
