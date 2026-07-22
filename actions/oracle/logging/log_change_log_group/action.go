// Package oracle_logging_log_change_log_group moves a log from one log group to another. The log
// keeps its OCID; only its parent log group changes. Asynchronous: returns a work-request id —
// poll until the move completes before relying on the new placement.
package oracle_logging_log_change_log_group

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: Change Log Log Group"
	Description  = "Move a log into a different log group — the log keeps its OCID, only its parent log group changes. Returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
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
	{Name: "log_group_ocid", Type: core.ConnectionTypeString, Label: "Log Group OCID", Placeholder: "ocid1.loggroup.oc1..aaaa… (the log's current group)", Required: true},
	{Name: "log_ocid", Type: core.ConnectionTypeString, Label: "Log OCID", Placeholder: "ocid1.log.oc1..aaaa… (the log to move)", Required: true},
	{Name: "target_log_group_ocid", Type: core.ConnectionTypeString, Label: "Target Log Group OCID", Placeholder: "ocid1.loggroup.oc1..aaaa… (where to move the log)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Log OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lg.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	logGroupID, err := lg.RequiredString("log_group_ocid", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	logID, err := lg.RequiredString("log_ocid", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	targetLogGroupID, err := lg.RequiredString("target_log_group_ocid", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}

	resp, err := client.ChangeLogLogGroup(lg.Context(), logging.ChangeLogLogGroupRequest{
		LogGroupId: &logGroupID,
		LogId:      &logID,
		ChangeLogLogGroupDetails: logging.ChangeLogLogGroupDetails{
			TargetLogGroupId: &targetLogGroupID,
		},
	})
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}

	return lg.Result(fmt.Sprintf("Moving log %s into log group %s", logID, targetLogGroupID), map[string]interface{}{
		"id":              logID,
		"work_request_id": lg.Str(resp.OpcWorkRequestId),
	}), nil
}
