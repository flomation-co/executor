// Package oracle_dns_resolver_endpoint_list lists the endpoints of a DNS resolver.
package oracle_dns_resolver_endpoint_list

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
	Name         = "OCI DNS: List Resolver Endpoints"
	Description  = "List the endpoints of an Oracle Cloud DNS resolver, optionally filtered by name and GLOBAL/PRIVATE scope. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the resolver picker)"},
	{Name: "resolver_ocid", Type: core.ConnectionTypeString, Label: "Resolver OCID", Placeholder: "ocid1.dnsresolver.oc1..aaaa… — the resolver whose endpoints to list", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Only the endpoint with this exact name (optional)"},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE (optional)", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resolver_endpoints", Type: core.ConnectionTypeObject, Label: "Resolver Endpoints"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, resolverID, errResult := dnsn.ResourceClient(inputs, "resolver_ocid")
	if errResult != nil {
		return errResult, nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.ListResolverEndpointsRequest{ResolverId: &resolverID}
	if scope != "" {
		req.Scope = dns.ListResolverEndpointsScopeEnum(scope)
	}
	if v := strings.TrimSpace(dnsn.OptionalString("name", inputs)); v != "" {
		req.Name = &v
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= dnsn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListResolverEndpoints(dnsn.Context(), req)
		if err != nil {
			return dnsn.ErrorResult(auth.OCIError(err)), nil
		}
		for _, item := range resp.Items {
			out = append(out, dnsn.SummariseResolverEndpointSummary(item))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Found %d resolver endpoint(s)", len(out)),
		"resolver_endpoints": out,
		"count":              fmt.Sprintf("%d", len(out)),
		"truncated":          truncated,
		"success":            true,
	}, nil
}
