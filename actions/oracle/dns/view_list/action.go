// Package oracle_dns_view_list lists the DNS views in a compartment.
package oracle_dns_view_list

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
	Name         = "OCI DNS: List Views"
	Description  = "List the Oracle Cloud DNS views in a compartment, optionally filtered by display name and GLOBAL/PRIVATE scope. Walks pagination up to a safe cap."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Only views with this exact display name (optional)"},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE (optional)", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "views", Type: core.ConnectionTypeObject, Label: "Views"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.ListViewsRequest{CompartmentId: &compartment}
	if scope != "" {
		req.Scope = dns.ListViewsScopeEnum(scope)
	}
	if v := strings.TrimSpace(dnsn.OptionalString("display_name", inputs)); v != "" {
		req.DisplayName = &v
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= dnsn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListViews(dnsn.Context(), req)
		if err != nil {
			return dnsn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, dnsn.SummariseViewSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d view(s)", len(out)),
		"views":       out,
		"count":       fmt.Sprintf("%d", len(out)),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
