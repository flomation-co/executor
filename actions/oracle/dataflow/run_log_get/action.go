// Package oracle_dataflow_run_log_get fetches the content of a single named log file produced by a
// Data Flow run, returning it as text.
package oracle_dataflow_run_log_get

import (
	"fmt"
	"io"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Get Run Log"
	Description  = "Fetch the content of a single named log file from a Data Flow run, returned as text."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "run_ocid", Type: core.ConnectionTypeString, Label: "Run OCID", Placeholder: "ocid1.dataflowrun.oc1..aaaa…", Required: true},
	{Name: "log_name", Type: core.ConnectionTypeString, Label: "Log File Name", Placeholder: "e.g. spark_driver_stdout.log.gz", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Log Content"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "content_length", Type: core.ConnectionTypeString, Label: "Content Length"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	runID, err := df.RequiredString("run_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	logName, err := df.RequiredString("log_name", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetRunLog(df.Context(), dataflow.GetRunLogRequest{RunId: &runID, Name: &logName})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}

	var content string
	if resp.Content != nil {
		body, readErr := io.ReadAll(resp.Content)
		_ = resp.Content.Close()
		if readErr != nil {
			return df.ErrorResult(fmt.Sprintf("read log content: %s", readErr.Error())), nil
		}
		content = string(body)
	}

	return df.Result(fmt.Sprintf("Fetched log %q for run %s (%d bytes)", logName, runID, len(content)), map[string]interface{}{
		"content":        content,
		"content_type":   df.Str(resp.ContentType),
		"content_length": fmt.Sprintf("%d", len(content)),
	}), nil
}
