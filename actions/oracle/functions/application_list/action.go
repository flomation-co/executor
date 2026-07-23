// Package oracle_functions_application_list lists the OCI Functions applications in a compartment,
// optionally filtered by display name, application OCID, or lifecycle state.
package oracle_functions_application_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: List Applications"
	Description  = "List the Oracle Cloud Functions applications in a compartment, optionally filtered by display name, application OCID, or lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name filter", Placeholder: "Only applications with this exact name (optional)"},
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID filter", Placeholder: "Only the application with this OCID (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State filter", Placeholder: "Filter by state (optional)", Options: []core.ConnectionOption{
		{Name: "Any (all states)", Value: ""},
		{Name: "Creating", Value: "CREATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Inactive", Value: "INACTIVE"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page, 1-50 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "applications", Type: core.ConnectionTypeObject, Label: "Applications"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}

	req := functions.ListApplicationsRequest{CompartmentId: &compartment}
	if dn := fn.OptionalString("display_name", inputs); dn != "" {
		req.DisplayName = &dn
	}
	if id := fn.OptionalString("application_ocid", inputs); id != "" {
		req.Id = &id
	}
	// The SDK's lifecycle filter is a typed enum, but OCI accepts the plain
	// upper-case string on the wire; pass it through so a new state Oracle adds
	// isn't gated by our build.
	if ls := strings.ToUpper(strings.TrimSpace(fn.OptionalString("lifecycle_state", inputs))); ls != "" {
		req.LifecycleState = functions.ApplicationLifecycleStateEnum(ls)
	}
	if n, ok, err := fn.OptionalInt64("limit", inputs); err != nil {
		return fn.ErrorResult(err.Error()), nil
	} else if ok {
		limit := int(n)
		req.Limit = &limit
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListApplications(fn.Context(), req)
		if err != nil {
			return fn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, fn.SummariseApplicationSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return fn.Result(fmt.Sprintf("Found %d application(s)", len(out)), map[string]interface{}{
		"applications": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
