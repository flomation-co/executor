// Package oracle_waf_web_app_firewall_list lists the Web Application Firewalls in a compartment.
// Optional filters narrow the result to an exact display name or a lifecycle state. Walks
// pagination up to a safe cap and flags when the list was truncated.
package oracle_waf_web_app_firewall_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: List Web App Firewalls"
	Description  = "List the Web Application Firewalls in a compartment. Optionally filter by exact display name or lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only firewalls with this exact display name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only firewalls in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Creating", Value: "CREATING"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max results per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "web_app_firewalls", Type: core.ConnectionTypeObject, Label: "Web App Firewalls"},
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

	req := waf.ListWebAppFirewallsRequest{CompartmentId: &compartment}
	if name := wf.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := wf.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = []waf.WebAppFirewallLifecycleStateEnum{waf.WebAppFirewallLifecycleStateEnum(state)}
	}
	if n, ok, err := wf.OptionalInt("limit", inputs); err != nil {
		return wf.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= wf.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWebAppFirewalls(wf.Context(), req)
		if err != nil {
			return wf.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, wf.SummariseWebAppFirewallSummary(resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return wf.Result(fmt.Sprintf("Found %d web app firewall(s)", len(out)), map[string]interface{}{
		"web_app_firewalls": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
