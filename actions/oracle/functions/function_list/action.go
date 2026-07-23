// Package oracle_functions_function_list lists the functions belonging to an application.
package oracle_functions_function_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: List Functions"
	Description  = "List the functions belonging to an Oracle Cloud application, with optional display-name, OCID and lifecycle-state filters. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "application_id", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.fnapp.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only functions with this exact display name (optional)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Function OCID Filter", Placeholder: "Only the function with this OCID (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only functions in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Inactive", Value: "INACTIVE"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page, 1–50 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "functions", Type: core.ConnectionTypeObject, Label: "Functions"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	applicationID, err := fn.RequiredString("application_id", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	req := functions.ListFunctionsRequest{ApplicationId: &applicationID}
	if displayName := fn.OptionalString("display_name", inputs); displayName != "" {
		req.DisplayName = &displayName
	}
	if id := fn.OptionalString("id", inputs); id != "" {
		req.Id = &id
	}
	if state := fn.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = functions.FunctionLifecycleStateEnum(state)
	}
	if limit, ok, err := fn.OptionalInt64("limit", inputs); err != nil {
		return fn.ErrorResult(err.Error()), nil
	} else if ok {
		l := int(limit)
		req.Limit = &l
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListFunctions(fn.Context(), req)
		if err != nil {
			return fn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, fn.SummariseFunctionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return fn.Result(fmt.Sprintf("Found %d function(s)", len(out)), map[string]interface{}{
		"functions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
