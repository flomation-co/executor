// Package oracle_waf_policy_update applies a partial update to a Web Application Firewall policy:
// only the display name you supply is changed, and blank fields are left unchanged. Asynchronous —
// the update returns a work-request id; poll the Get Policy action until the policy is ACTIVE.
package oracle_waf_policy_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Update Policy"
	Description  = "Partially update a Web Application Firewall policy — rename it via the display name you supply; blank fields are left unchanged. Asynchronous: returns a work-request id, poll Get Policy until ACTIVE."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Policy OCID", Placeholder: "ocid1.webappfirewallpolicy.oc1..aaaa… — the policy to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Policy OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wf.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	policyID, err := wf.RequiredString("policy_ocid", inputs)
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied. A blank display name is
	// left nil so the policy keeps its current name (shallow merge leaves undefined fields unchanged).
	details := waf.UpdateWebAppFirewallPolicyDetails{}
	if v := wf.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}

	resp, err := client.UpdateWebAppFirewallPolicy(wf.Context(), waf.UpdateWebAppFirewallPolicyRequest{
		WebAppFirewallPolicyId:            &policyID,
		UpdateWebAppFirewallPolicyDetails: details,
	})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}
	return wf.Result(fmt.Sprintf("Updating WAF policy %s — poll Get Policy until ACTIVE", policyID), map[string]interface{}{
		"id":              policyID,
		"work_request_id": wf.Str(resp.OpcWorkRequestId),
	}), nil
}
