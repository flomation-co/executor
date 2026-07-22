// Package oracle_dataflow_statement_list lists the interactive statements submitted against a Data
// Flow SESSION run, optionally filtered by lifecycle state. Walks pagination up to a safe cap.
package oracle_dataflow_statement_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: List Statements"
	Description  = "List the interactive statements submitted against a Data Flow session run. Optionally filter by lifecycle state. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "run_ocid", Type: core.ConnectionTypeString, Label: "Run OCID", Placeholder: "ocid1.dataflowrun.oc1..aaaa… (the session run to list statements for)", Required: true},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter to statements in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Accepted", Value: "ACCEPTED"},
		{Name: "In Progress", Value: "IN_PROGRESS"},
		{Name: "Succeeded", Value: "SUCCEEDED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Cancelling", Value: "CANCELLING"},
		{Name: "Cancelled", Value: "CANCELLED"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "statements", Type: core.ConnectionTypeObject, Label: "Statements"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	req := dataflow.ListStatementsRequest{RunId: &runID}
	if state := df.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = dataflow.ListStatementsLifecycleStateEnum(state)
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= df.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListStatements(df.Context(), req)
		if err != nil {
			return df.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, df.SummariseStatementSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return df.Result(fmt.Sprintf("Found %d statement(s)", len(out)), map[string]interface{}{
		"statements": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
