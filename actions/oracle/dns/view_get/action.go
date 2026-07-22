// Package oracle_dns_view_get reads one private-DNS view by OCID.
package oracle_dns_view_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Get View"
	Description  = "Fetch a single Oracle Cloud private-DNS view by OCID — the namespace of zones a resolver serves, with its lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+layer-group"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the view picker)"},
	{Name: "view_ocid", Type: core.ConnectionTypeString, Label: "View OCID", Placeholder: "ocid1.dns-view.oc1..aaaa…", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL or PRIVATE (optional)", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "view", Type: core.ConnectionTypeObject, Label: "View"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "View OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "view_ocid")
	if errResult != nil {
		return errResult, nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.GetViewRequest{ViewId: &id}
	if scope != "" {
		req.Scope = dns.GetViewScopeEnum(scope)
	}
	resp, err := client.GetView(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	view := dnsn.SummariseView(&resp.View)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("View %q is %s", view["display_name"], view["lifecycle_state"]),
		"view":        view,
		"id":          view["id"],
		"success":     true,
	}, nil
}
