// Package oracle_dataflow_run_list lists the Data Flow runs in a compartment, optionally filtered by
// application, exact display name or lifecycle state. Walks pagination up to a safe cap.
package oracle_dataflow_run_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: List Runs"
	Description  = "List the Spark runs in a compartment. Optionally filter by application, exact display name or lifecycle state, and cap the page size. Walks pagination up to a safe cap."
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
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID Filter", Placeholder: "Only runs of this application (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only runs with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by run state (optional)", Options: []core.ConnectionOption{
		{Name: "Accepted", Value: "ACCEPTED"},
		{Name: "In Progress", Value: "IN_PROGRESS"},
		{Name: "Canceling", Value: "CANCELING"},
		{Name: "Canceled", Value: "CANCELED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Succeeded", Value: "SUCCEEDED"},
		{Name: "Stopping", Value: "STOPPING"},
		{Name: "Stopped", Value: "STOPPED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max results per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "runs", Type: core.ConnectionTypeObject, Label: "Runs"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	req := dataflow.ListRunsRequest{CompartmentId: &compartment}
	if appID := df.OptionalString("application_ocid", inputs); appID != "" {
		req.ApplicationId = &appID
	}
	if name := df.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := df.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = dataflow.ListRunsLifecycleStateEnum(state)
	}
	if limit, ok, err := df.OptionalInt("limit", inputs); err != nil {
		return df.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= df.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListRuns(df.Context(), req)
		if err != nil {
			return df.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, df.SummariseRunSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return df.Result(fmt.Sprintf("Found %d run(s)", len(out)), map[string]interface{}{
		"runs": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
