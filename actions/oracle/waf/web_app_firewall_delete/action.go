// Package oracle_waf_web_app_firewall_delete deletes a Web Application Firewall by its OCID,
// detaching it from its backend. Asynchronous: returns a work-request id — poll it to confirm.
package oracle_waf_web_app_firewall_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Delete Web App Firewall"
	Description  = "Delete a Web Application Firewall by its OCID — it detaches from its backend. Returns a work-request id to poll for completion."
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
	{Name: "web_app_firewall_ocid", Type: core.ConnectionTypeString, Label: "Web App Firewall OCID", Placeholder: "ocid1.webappfirewall.oc1..aaaa… of the firewall to delete", Required: true},
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

	resp, err := client.DeleteWebAppFirewall(wf.Context(), waf.DeleteWebAppFirewallRequest{WebAppFirewallId: &firewallID})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}
	return wf.Result(fmt.Sprintf("Deleting web app firewall %s — poll the work request for completion", firewallID), map[string]interface{}{
		"id":              firewallID,
		"work_request_id": wf.Str(resp.OpcWorkRequestId),
	}), nil
}
