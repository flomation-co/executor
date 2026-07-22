// Package oracle_dns_view_create creates a private-DNS view — the container that holds
// private zones for a set of VCNs. Synchronous.
package oracle_dns_view_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Create View"
	Description  = "Create a private-DNS view in a compartment — the container that holds private zones. Private DNS only (PRIVATE scope)."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the view", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "view", Type: core.ConnectionTypeObject, Label: "View"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "View OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	displayName, err := dnsn.RequiredString("display_name", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	details := dns.CreateViewDetails{CompartmentId: &compartment}
	if v := strings.TrimSpace(displayName); v != "" {
		details.DisplayName = &v
	}
	// Views are a private-DNS construct; the create request carries the PRIVATE scope.
	resp, err := client.CreateView(dnsn.Context(), dns.CreateViewRequest{CreateViewDetails: details, Scope: dns.CreateViewScopePrivate})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	view := dnsn.SummariseView(&resp.View)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created view %q", displayName),
		"view":        view,
		"id":          view["id"],
		"success":     true,
	}, nil
}
