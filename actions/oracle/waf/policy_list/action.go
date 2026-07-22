// Package oracle_waf_policy_list lists the Web Application Firewall policies in a compartment,
// optionally filtered by display name and lifecycle state, walking pagination up to a safe cap.
package oracle_waf_policy_list

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: List Policies"
	Description  = "List the Web Application Firewall policies in a compartment, optionally filtered by exact display name and lifecycle state. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-virus"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only policies with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only policies in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page, 1–1000 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policies", Type: core.ConnectionTypeObject, Label: "Policies"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wf.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}
	req := waf.ListWebAppFirewallPoliciesRequest{CompartmentId: &compartment}
	if dn := wf.OptionalString("display_name", inputs); dn != "" {
		req.DisplayName = &dn
	}
	if state := strings.TrimSpace(wf.OptionalString("lifecycle_state", inputs)); state != "" {
		req.LifecycleState = []waf.WebAppFirewallPolicyLifecycleStateEnum{waf.WebAppFirewallPolicyLifecycleStateEnum(strings.ToUpper(state))}
	}
	if raw := strings.TrimSpace(wf.OptionalString("limit", inputs)); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 1000 {
			return wf.ErrorResult("page size must be a whole number between 1 and 1000"), nil
		}
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= wf.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWebAppFirewallPolicies(wf.Context(), req)
		if err != nil {
			return wf.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, wf.SummariseWebAppFirewallPolicySummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return wf.Result(fmt.Sprintf("Found %d WAF policy/policies", len(out)), map[string]interface{}{
		"policies": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
