// Package oracle_monitoring_alarm_suppression_list lists the alarm suppressions in a compartment,
// optionally narrowed to a single alarm. A suppression silences an alarm (whole or by dimension)
// for a window. Walks pagination up to a safe cap.
package oracle_monitoring_alarm_suppression_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: List Alarm Suppressions"
	Description  = "List the alarm suppressions in a compartment, optionally filtered to a single alarm by its OCID. Each suppression silences an alarm — whole or by dimension — for a time window. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "alarm_ocid", Type: core.ConnectionTypeString, Label: "Alarm OCID Filter", Placeholder: "Only suppressions targeting this alarm (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm_suppressions", Type: core.ConnectionTypeObject, Label: "Alarm Suppressions"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := mon.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	// OCI accepts EXACTLY ONE of alarmId or compartmentId — passing both is a 400
	// ("Only alarmId or compartmentId is required"). Prefer the more specific alarm filter when
	// given; otherwise scope to the compartment.
	req := monitoring.ListAlarmSuppressionsRequest{}
	if alarm := mon.OptionalString("alarm_ocid", inputs); alarm != "" {
		req.AlarmId = &alarm
	} else {
		compartment, err := auth.RequiredCompartment()
		if err != nil {
			return mon.ErrorResult(err.Error()), nil
		}
		req.CompartmentId = &compartment
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= mon.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListAlarmSuppressions(mon.Context(), req)
		if err != nil {
			return mon.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseAlarmSuppression(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return mon.Result(fmt.Sprintf("Found %d alarm suppression(s)", len(out)), map[string]interface{}{
		"alarm_suppressions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}

func summariseAlarmSuppression(s *monitoring.AlarmSuppressionSummary) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  mon.Str(s.Id),
		"compartment_id":      mon.Str(s.CompartmentId),
		"display_name":        mon.Str(s.DisplayName),
		"level":               string(s.Level),
		"lifecycle_state":     string(s.LifecycleState),
		"description":         mon.Str(s.Description),
		"dimensions":          s.Dimensions,
		"time_suppress_from":  mon.FormatTime(s.TimeSuppressFrom),
		"time_suppress_until": mon.FormatTime(s.TimeSuppressUntil),
		"time_created":        mon.FormatTime(s.TimeCreated),
		"time_updated":        mon.FormatTime(s.TimeUpdated),
	}
	switch t := s.AlarmSuppressionTarget.(type) {
	case monitoring.AlarmSuppressionAlarmTarget:
		m["target_type"] = "ALARM"
		m["target_alarm_id"] = mon.Str(t.AlarmId)
	case monitoring.AlarmSuppressionCompartmentTarget:
		m["target_type"] = "COMPARTMENT"
		m["target_compartment_id"] = mon.Str(t.CompartmentId)
		m["target_compartment_id_in_subtree"] = mon.Bool(t.CompartmentIdInSubtree)
	}
	return m
}
