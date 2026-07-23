// Package oracle_language_endpoint_list lists the OCI Language endpoints in a compartment, optionally
// filtered by project, exact display name or lifecycle state. Walks pagination up to a safe cap.
package oracle_language_endpoint_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: List Endpoints"
	Description  = "List the OCI Language endpoints in a compartment. Optionally filter by project, exact display name or lifecycle state, and cap the number of items. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project OCID Filter", Placeholder: "Only endpoints in this project (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only endpoints with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"}, {Name: "Active", Value: "ACTIVE"}, {Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"}, {Name: "Deleted", Value: "DELETED"}, {Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoints", Type: core.ConnectionTypeObject, Label: "Endpoints"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}
	req := ailanguage.ListEndpointsRequest{CompartmentId: &compartment}
	if projectID := lang.OptionalString("project_id", inputs); projectID != "" {
		req.ProjectId = &projectID
	}
	if name := lang.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := lang.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = ailanguage.EndpointLifecycleStateEnum(state)
	}
	if limit, ok, err := lang.OptionalInt("limit", inputs); err != nil {
		return lang.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= lang.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListEndpoints(lang.Context(), req)
		if err != nil {
			return lang.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, lang.SummariseEndpointSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return lang.Result(fmt.Sprintf("Found %d endpoint(s)", len(out)), map[string]interface{}{
		"endpoints": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
