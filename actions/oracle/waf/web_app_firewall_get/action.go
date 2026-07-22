// Package oracle_waf_web_app_firewall_get fetches a single Web Application Firewall by OCID,
// returning the policy it enforces, its backend and lifecycle state.
package oracle_waf_web_app_firewall_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Get Web App Firewall"
	Description  = "Fetch a single Web Application Firewall by its OCID — its policy, backend and lifecycle state."
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
	{Name: "web_app_firewall_ocid", Type: core.ConnectionTypeString, Label: "Web App Firewall OCID", Placeholder: "ocid1.webappfirewall.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "web_app_firewall", Type: core.ConnectionTypeObject, Label: "Web App Firewall"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Web App Firewall OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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

	resp, err := client.GetWebAppFirewall(wf.Context(), waf.GetWebAppFirewallRequest{WebAppFirewallId: &firewallID})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}
	firewall := wf.SummariseWebAppFirewall(resp.WebAppFirewall)
	return wf.Result(fmt.Sprintf("Web App Firewall %q (%s)", firewall["display_name"], firewall["lifecycle_state"]), map[string]interface{}{
		"web_app_firewall": firewall, "id": firewall["id"], "lifecycle_state": firewall["lifecycle_state"],
	}), nil
}
