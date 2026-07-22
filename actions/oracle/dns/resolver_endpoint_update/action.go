// Package oracle_dns_resolver_endpoint_update updates a DNS resolver VNIC endpoint's
// network-security-group set (and preserves its tags) via read-modify-write.
package oracle_dns_resolver_endpoint_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Update Resolver Endpoint"
	Description  = "Update an Oracle Cloud DNS resolver endpoint's network security groups, preserving its existing tags and configuration."
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
	{Name: "resolver_ocid", Type: core.ConnectionTypeString, Label: "Resolver OCID", Placeholder: "ocid1.dnsresolver.oc1..aaaa… — the parent resolver", Required: true},
	{Name: "endpoint_name", Type: core.ConnectionTypeString, Label: "Endpoint Name", Placeholder: "The resolver endpoint's name", Required: true},
	{Name: "nsg_ocids", Type: core.ConnectionTypeString, Label: "Network Security Group OCIDs", Placeholder: "Comma-separated NSG OCIDs (leave blank to keep the current set)"},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private resolvers", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoint", Type: core.ConnectionTypeObject, Label: "Resolver Endpoint"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Endpoint Name"},
	{Name: "resolver_id", Type: core.ConnectionTypeString, Label: "Resolver OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	resolverID, name, err := dnsn.ResolverEndpointName(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}

	// READ: fetch the current endpoint so we preserve everything the update body carries.
	getReq := dns.GetResolverEndpointRequest{ResolverId: &resolverID, ResolverEndpointName: &name}
	if scope != "" {
		getReq.Scope = dns.GetResolverEndpointScopeEnum(scope)
	}
	getResp, err := client.GetResolverEndpoint(dnsn.Context(), getReq)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	current := getResp.ResolverEndpoint

	// MODIFY: seed the (full-replace) update body from the current values...
	details := dns.UpdateResolverVnicEndpointDetails{
		FreeformTags: current.GetFreeformTags(),
		DefinedTags:  current.GetDefinedTags(),
	}
	if vnic, ok := current.(dns.ResolverVnicEndpoint); ok {
		details.NsgIds = vnic.NsgIds
		// SecurityAttributes has no omitempty, so a nil value serialises to null and
		// would WIPE any ZPR attributes on this full-replace PUT — re-send the current.
		details.SecurityAttributes = vnic.SecurityAttributes
	}
	// ...then overlay the operator-supplied NSG set only when they provided one.
	if dnsn.BoolWasSet("nsg_ocids", inputs) {
		details.NsgIds = dnsn.InputStrings("nsg_ocids", inputs)
	}

	// WRITE.
	req := dns.UpdateResolverEndpointRequest{
		ResolverId:                    &resolverID,
		ResolverEndpointName:          &name,
		UpdateResolverEndpointDetails: details,
	}
	if scope != "" {
		req.Scope = dns.UpdateResolverEndpointScopeEnum(scope)
	}
	resp, err := client.UpdateResolverEndpoint(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}

	endpoint := dnsn.SummariseResolverEndpoint(resp.ResolverEndpoint)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Resolver endpoint %q updated (%s)", endpoint["name"], endpoint["lifecycle_state"]),
		"endpoint":        endpoint,
		"name":            endpoint["name"],
		"resolver_id":     endpoint["resolver_id"],
		"lifecycle_state": endpoint["lifecycle_state"],
		// UpdateResolverEndpoint is async (202) — surface the work-request id so a flow
		// can poll for completion, like the create/delete siblings.
		"work_request_id": dnsn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
