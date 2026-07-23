// Package oracle_monitoring_alarm_create creates an alarm: a Monitoring Query Language (MQL)
// expression evaluated against a metric that, when it breaches, fires to one or more destinations
// (Notifications topics). Synchronous — returns the created alarm.
package oracle_monitoring_alarm_create

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
	Name         = "OCI Monitoring: Create Alarm"
	Description  = "Create an alarm from an MQL query over a metric namespace. When the query breaches for the pending duration, the alarm fires to its destinations (Notifications topic OCIDs). Severity is CRITICAL/ERROR/WARNING/INFO."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where the alarm lives)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the alarm", Required: true},
	{Name: "metric_compartment_ocid", Type: core.ConnectionTypeString, Label: "Metric Compartment OCID", Placeholder: "Compartment whose metrics to watch (defaults to the alarm compartment)"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Metric Namespace", Placeholder: "e.g. oci_computeagent", Required: true},
	{Name: "query", Type: core.ConnectionTypeText, Label: "MQL Query", Placeholder: "e.g. CpuUtilization[1m].mean() > 90", Required: true},
	{Name: "severity", Type: core.ConnectionTypeString, Label: "Severity", Required: true, Options: []core.ConnectionOption{
		{Name: "Critical", Value: "CRITICAL"}, {Name: "Error", Value: "ERROR"}, {Name: "Warning", Value: "WARNING"}, {Name: "Info", Value: "INFO"},
	}},
	{Name: "destinations", Type: core.ConnectionTypeString, Label: "Destinations (topic OCIDs)", Placeholder: "Comma-separated Notifications topic OCIDs", Required: true},
	{Name: "pending_duration", Type: core.ConnectionTypeString, Label: "Pending Duration", Placeholder: "ISO-8601, e.g. PT5M — breach must persist this long (optional)"},
	{Name: "resolution", Type: core.ConnectionTypeString, Label: "Resolution", Placeholder: "e.g. 1m (optional)"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Notification Body", Placeholder: "Message sent when the alarm fires (optional)"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Enable the alarm now (default true)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	name, err := mon.RequiredString("display_name", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	namespace, err := mon.RequiredString("namespace", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	query, err := mon.RequiredString("query", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	severity, err := mon.RequiredString("severity", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	destRaw, err := mon.RequiredString("destinations", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	var destinations []string
	for _, d := range strings.Split(destRaw, ",") {
		if d = strings.TrimSpace(d); d != "" {
			destinations = append(destinations, d)
		}
	}
	if len(destinations) == 0 {
		return mon.ErrorResult("at least one destination (Notifications topic OCID) is required"), nil
	}
	metricComp := mon.OptionalString("metric_compartment_ocid", inputs)
	if metricComp == "" {
		metricComp = compartment
	}
	enabled := true
	if p := mon.OptionalBoolPtr("is_enabled", inputs); p != nil {
		enabled = *p
	}

	details := monitoring.CreateAlarmDetails{
		DisplayName:         &name,
		CompartmentId:       &compartment,
		MetricCompartmentId: &metricComp,
		Namespace:           &namespace,
		Query:               &query,
		Severity:            monitoring.AlarmSeverityEnum(severity),
		Destinations:        destinations,
		IsEnabled:           &enabled,
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
	if tags, err := mon.FreeformTags("freeform_tags", inputs); err != nil {
		return mon.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.CreateAlarm(mon.Context(), monitoring.CreateAlarmRequest{CreateAlarmDetails: details})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}
	alarm := mon.SummariseAlarm(&resp.Alarm)
	return mon.Result(fmt.Sprintf("Created alarm %q (%s)", alarm["display_name"], alarm["lifecycle_state"]), map[string]interface{}{
		"alarm": alarm, "id": alarm["id"], "lifecycle_state": alarm["lifecycle_state"],
	}), nil
}
