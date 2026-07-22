// Package oracle_monitoring_alarm_get fetches a single alarm by OCID and returns its full
// definition (MQL query, severity, destinations, lifecycle state). Synchronous read.
package oracle_monitoring_alarm_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Get Alarm"
	Description  = "Fetch a single alarm by its OCID, returning its MQL query, severity, destinations, and lifecycle state."
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
	{Name: "alarm_ocid", Type: core.ConnectionTypeString, Label: "Alarm OCID", Placeholder: "ocid1.alarm.oc1..aaaa… — the alarm to fetch", Required: true},
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

	resp, err := client.GetAlarm(mon.Context(), monitoring.GetAlarmRequest{AlarmId: &alarmID})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}
	alarm := mon.SummariseAlarm(&resp.Alarm)
	return mon.Result(fmt.Sprintf("Fetched alarm %q (%s)", alarm["display_name"], alarm["lifecycle_state"]), map[string]interface{}{
		"alarm": alarm, "id": alarm["id"], "lifecycle_state": alarm["lifecycle_state"],
	}), nil
}
