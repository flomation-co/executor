// Package oracle_monitoring_alarms_status_list lists the current evaluation status (FIRING/OK/
// SUSPENDED) of the alarms in a compartment. Walks pagination up to a safe cap.
package oracle_monitoring_alarms_status_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: List Alarm Statuses"
	Description  = "List the current evaluation status (FIRING, OK or SUSPENDED) of the alarms in a compartment, optionally filtered to a single alarm by exact display name. Walks pagination up to a safe cap."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only the alarm with this exact display name (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm_statuses", Type: core.ConnectionTypeObject, Label: "Alarm Statuses"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	req := monitoring.ListAlarmsStatusRequest{CompartmentId: &compartment}
	if name := mon.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if limit, ok, err := mon.OptionalInt("limit", inputs); err != nil {
		return mon.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= mon.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListAlarmsStatus(mon.Context(), req)
		if err != nil {
			return mon.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, mon.SummariseAlarmStatus(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return mon.Result(fmt.Sprintf("Found %d alarm status(es)", len(out)), map[string]interface{}{
		"alarm_statuses": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
