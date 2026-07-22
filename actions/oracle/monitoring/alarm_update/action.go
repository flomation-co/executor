// Package oracle_monitoring_alarm_update updates an existing alarm. It is a partial update: only
// the fields the operator supplies are sent, so blank inputs leave the alarm's current values
// untouched. Synchronous — returns the updated alarm.
package oracle_monitoring_alarm_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Update Alarm"
	Description  = "Update an existing alarm by OCID. Only the fields you fill in are changed — leave a field blank to keep its current value. Severity is CRITICAL/ERROR/WARNING/INFO; destinations are comma-separated Notifications topic OCIDs."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where the alarm lives)", Required: true},
	{Name: "alarm_ocid", Type: core.ConnectionTypeString, Label: "Alarm OCID", Placeholder: "ocid1.alarm.oc1..aaaa… (the alarm to update)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name for the alarm (leave blank to keep)"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Metric Namespace", Placeholder: "e.g. oci_computeagent (leave blank to keep)"},
	{Name: "query", Type: core.ConnectionTypeText, Label: "MQL Query", Placeholder: "e.g. CpuUtilization[1m].mean() > 90 (leave blank to keep)"},
	{Name: "severity", Type: core.ConnectionTypeString, Label: "Severity", Options: []core.ConnectionOption{
		{Name: "(unchanged)", Value: ""}, {Name: "Critical", Value: "CRITICAL"}, {Name: "Error", Value: "ERROR"}, {Name: "Warning", Value: "WARNING"}, {Name: "Info", Value: "INFO"},
	}},
	{Name: "destinations", Type: core.ConnectionTypeString, Label: "Destinations (topic OCIDs)", Placeholder: "Comma-separated Notifications topic OCIDs (leave blank to keep)"},
	{Name: "pending_duration", Type: core.ConnectionTypeString, Label: "Pending Duration", Placeholder: "ISO-8601, e.g. PT5M (leave blank to keep)"},
	{Name: "resolution", Type: core.ConnectionTypeString, Label: "Resolution", Placeholder: "e.g. 1m (leave blank to keep)"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Notification Body", Placeholder: "Message sent when the alarm fires (leave blank to keep)"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Enable or disable the alarm (leave blank to keep)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (leave blank to keep)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm", Type: core.ConnectionTypeObject, Label: "Alarm"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Alarm OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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

	details := monitoring.UpdateAlarmDetails{}
	if v := mon.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := mon.OptionalString("namespace", inputs); v != "" {
		details.Namespace = &v
	}
	if v := mon.OptionalString("query", inputs); v != "" {
		details.Query = &v
	}
	if v := strings.TrimSpace(mon.OptionalString("severity", inputs)); v != "" {
		details.Severity = monitoring.AlarmSeverityEnum(v)
	}
	if v := mon.OptionalString("destinations", inputs); strings.TrimSpace(v) != "" {
		var destinations []string
		for _, d := range strings.Split(v, ",") {
			if d = strings.TrimSpace(d); d != "" {
				destinations = append(destinations, d)
			}
		}
		details.Destinations = destinations
	}
	if v := mon.OptionalString("pending_duration", inputs); v != "" {
		details.PendingDuration = &v
	}
	if v := mon.OptionalString("resolution", inputs); v != "" {
		details.Resolution = &v
	}
	if v := mon.OptionalString("body", inputs); v != "" {
		details.Body = &v
	}
	if p := mon.OptionalBoolPtr("is_enabled", inputs); p != nil {
		details.IsEnabled = p
	}
	if tags, err := mon.FreeformTags("freeform_tags", inputs); err != nil {
		return mon.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateAlarm(mon.Context(), monitoring.UpdateAlarmRequest{
		AlarmId:            &alarmID,
		UpdateAlarmDetails: details,
	})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}
	alarm := mon.SummariseAlarm(&resp.Alarm)
	return mon.Result(fmt.Sprintf("Updated alarm %q (%s)", alarm["display_name"], alarm["lifecycle_state"]), map[string]interface{}{
		"alarm": alarm, "id": alarm["id"], "lifecycle_state": alarm["lifecycle_state"],
	}), nil
}
