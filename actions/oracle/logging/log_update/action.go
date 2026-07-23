// Package oracle_logging_log_update applies a partial update to a log: only the display name,
// enabled flag and retention duration you supply are changed; blank fields are left unchanged.
// Asynchronous — the update returns a work-request id; poll Get Log until the change lands.
package oracle_logging_log_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: Update Log"
	Description  = "Partially update a log — change only the display name, enabled flag or retention duration you supply; blank fields are left unchanged. Returns a work-request id — poll Get Log until the change lands."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "log_group_ocid", Type: core.ConnectionTypeString, Label: "Log Group OCID", Placeholder: "ocid1.loggroup.oc1..aaaa… — the enclosing log group", Required: true},
	{Name: "log_ocid", Type: core.ConnectionTypeString, Label: "Log OCID", Placeholder: "ocid1.log.oc1..aaaa… — the log to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Enable/disable the log (leave blank to keep unchanged)"},
	{Name: "retention_duration", Type: core.ConnectionTypeString, Label: "Retention Duration (days)", Placeholder: "30-day increments: 30, 60, 90 … up to 180 (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Log OCID"},
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

	// Partial update: only carry the fields the operator actually supplied. is_enabled is a *bool
	// (nil = unchanged); retention_duration is only set when a whole number is given.
	details := logging.UpdateLogDetails{IsEnabled: lg.OptionalBoolPtr("is_enabled", inputs)}
	if v := lg.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if n, ok, err := lg.OptionalInt("retention_duration", inputs); err != nil {
		return lg.ErrorResult(err.Error()), nil
	} else if ok {
		details.RetentionDuration = &n
	}

	resp, err := client.UpdateLog(lg.Context(), logging.UpdateLogRequest{
		LogGroupId:       &logGroupID,
		LogId:            &logID,
		UpdateLogDetails: details,
	})
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}
	return lg.Result(fmt.Sprintf("Updating log %s — poll Get Log until the change lands", logID), map[string]interface{}{
		"work_request_id": lg.Str(resp.OpcWorkRequestId),
		"id":              logID,
	}), nil
}
