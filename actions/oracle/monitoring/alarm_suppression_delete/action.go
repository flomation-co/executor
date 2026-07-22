// Package oracle_monitoring_alarm_suppression_delete deletes an alarm suppression by OCID.
// A suppression temporarily silences an alarm (or a subset of its metric streams); deleting it
// lifts that silence. Synchronous — there is no response body, so success is the deletion itself.
package oracle_monitoring_alarm_suppression_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Delete Alarm Suppression"
	Description  = "Delete an alarm suppression by its OCID, lifting the silence it applied to the alarm. Synchronous — the suppression is removed immediately."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "alarm_suppression_ocid", Type: core.ConnectionTypeString, Label: "Alarm Suppression OCID", Placeholder: "ocid1.alarmsuppression.oc1..aaaa… (the suppression to delete)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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

	_, err = client.DeleteAlarmSuppression(mon.Context(), monitoring.DeleteAlarmSuppressionRequest{AlarmSuppressionId: &id})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}
	return mon.Result(fmt.Sprintf("Deleted alarm suppression %s", id), map[string]interface{}{
		"id": id,
	}), nil
}
