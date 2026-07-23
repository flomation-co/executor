// Package oracle_logging_log_create creates a log object inside a log group — either a CUSTOM log
// (you ingest into it via the ingestion API/agent) or a SERVICE log (OCI services emit into it).
// Asynchronous: OCI returns a work-request id; poll the work request until the log is ACTIVE.
package oracle_logging_log_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: Create Log"
	Description  = "Create a custom or service log inside a log group. Returns a work-request id — poll until the log is ACTIVE."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the log", Required: true},
	{Name: "log_type", Type: core.ConnectionTypeString, Label: "Log Type", Placeholder: "CUSTOM (you ingest) or SERVICE (an OCI service emits)", Required: true, Options: []core.ConnectionOption{
		{Name: "Custom", Value: "CUSTOM"},
		{Name: "Service", Value: "SERVICE"},
	}},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Whether the log is enabled (optional)"},
	{Name: "retention_duration", Type: core.ConnectionTypeString, Label: "Retention (days)", Placeholder: "In 30-day increments: 30, 60, 90 … up to 180 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	name, err := lg.RequiredString("display_name", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	rawType, err := lg.RequiredString("log_type", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	logType, ok := logging.GetMappingCreateLogDetailsLogTypeEnum(rawType)
	if !ok {
		return lg.ErrorResult(fmt.Sprintf("log type must be CUSTOM or SERVICE, got %q", rawType)), nil
	}

	details := logging.CreateLogDetails{
		DisplayName: &name,
		LogType:     logType,
		IsEnabled:   lg.OptionalBoolPtr("is_enabled", inputs),
	}
	if n, ok, err := lg.OptionalInt("retention_duration", inputs); err != nil {
		return lg.ErrorResult(err.Error()), nil
	} else if ok {
		details.RetentionDuration = &n
	}

	resp, err := client.CreateLog(lg.Context(), logging.CreateLogRequest{
		LogGroupId:       &logGroupID,
		CreateLogDetails: details,
	})
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}
	return lg.Result(fmt.Sprintf("Creating %s log %q — poll the work request until ACTIVE", logType, name), map[string]interface{}{
		"work_request_id": lg.Str(resp.OpcWorkRequestId),
	}), nil
}
