// Package oracle_waf_web_app_firewall_update applies a partial update to a Web Application Firewall:
// only the display name and/or attached policy you supply are changed; blank fields are left as-is.
// Asynchronous — the change comes back with a work-request id; poll Get Web App Firewall until the
// firewall settles in an ACTIVE state.
package oracle_waf_web_app_firewall_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Update Web App Firewall"
	Description  = "Partially update a Web Application Firewall — change only the display name and/or attached policy you supply; blank fields are left unchanged. Asynchronous: returns a work-request id, poll Get Web App Firewall until ACTIVE."
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
	{Name: "web_app_firewall_ocid", Type: core.ConnectionTypeString, Label: "Web App Firewall OCID", Placeholder: "ocid1.webappfirewall.oc1..aaaa… — the firewall to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Policy OCID", Placeholder: "ocid1.webappfirewallpolicy.oc1..aaaa… — new policy to attach (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Web App Firewall OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wf.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	firewallID, err := wf.RequiredString("web_app_firewall_ocid", inputs)
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied; blanks stay unchanged.
	details := waf.UpdateWebAppFirewallDetails{}
	if v := wf.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := wf.OptionalString("policy_ocid", inputs); v != "" {
		details.WebAppFirewallPolicyId = &v
	}

	resp, err := client.UpdateWebAppFirewall(wf.Context(), waf.UpdateWebAppFirewallRequest{
		WebAppFirewallId:            &firewallID,
		UpdateWebAppFirewallDetails: details,
	})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}
	return wf.Result(fmt.Sprintf("Updating web app firewall %s — poll Get Web App Firewall until ACTIVE", firewallID), map[string]interface{}{
		"id":              firewallID,
		"work_request_id": wf.Str(resp.OpcWorkRequestId),
	}), nil
}
