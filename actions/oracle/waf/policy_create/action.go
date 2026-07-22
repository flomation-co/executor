// Package oracle_waf_policy_create creates a Web Application Firewall policy — the reusable set of
// access-control, rate-limiting and protection rules a WebAppFirewall attaches to a backend. This
// keeps it minimal: an empty policy with just a display name, ready to be filled in later.
// Asynchronous: the policy comes back CREATING with a work-request id; poll Get Policy until ACTIVE.
package oracle_waf_policy_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: Create Policy"
	Description  = "Create an empty Web Application Firewall policy with a display name. Returns a work-request id — poll Get Policy until ACTIVE, then add rules."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the policy (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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

	details := waf.CreateWebAppFirewallPolicyDetails{CompartmentId: &compartment}
	if name := wf.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateWebAppFirewallPolicy(wf.Context(), waf.CreateWebAppFirewallPolicyRequest{
		CreateWebAppFirewallPolicyDetails: details,
	})
	if err != nil {
		return wf.ErrorResult(auth.OCIError(err)), nil
	}

	label := wf.Str(details.DisplayName)
	if label == "" {
		label = "policy"
	}
	return wf.Result(fmt.Sprintf("Creating WAF %s — poll Get Policy until ACTIVE", label), map[string]interface{}{
		"work_request_id": wf.Str(resp.OpcWorkRequestId),
	}), nil
}
