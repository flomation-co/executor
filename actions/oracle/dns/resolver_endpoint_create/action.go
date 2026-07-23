// Package oracle_dns_resolver_endpoint_create creates a VNIC resolver endpoint under an
// existing private-DNS resolver — a forwarding and/or listening address inside a subnet
// of the resolver's VCN. Provisioning is asynchronous: the call returns the endpoint (in
// a CREATING state) plus a work-request id; poll Get Resolver Endpoint until it is ACTIVE.
package oracle_dns_resolver_endpoint_create

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
	Name         = "OCI DNS: Create Resolver Endpoint"
	Description  = "Create a VNIC resolver endpoint under a private-DNS resolver — a forwarding and/or listening address in a subnet of the resolver's VCN. Returns the endpoint immediately plus a work-request id; poll Get Resolver Endpoint until it is ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the resolver picker)"},
	{Name: "resolver_ocid", Type: core.ConnectionTypeString, Label: "Resolver OCID", Placeholder: "ocid1.dnsresolver.oc1..aaaa… — the parent resolver", Required: true},
	{Name: "endpoint_name", Type: core.ConnectionTypeString, Label: "Endpoint Name", Placeholder: "Unique name within the resolver, e.g. forwarder-1", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… — must be in the resolver's VCN", Required: true},
	{Name: "is_forwarding", Type: core.ConnectionTypeBoolean, Label: "Is Forwarding", Placeholder: "Endpoint forwards outbound queries"},
	{Name: "is_listening", Type: core.ConnectionTypeBoolean, Label: "Is Listening", Placeholder: "Endpoint listens for inbound queries"},
	{Name: "forwarding_address", Type: core.ConnectionTypeString, Label: "Forwarding Address", Placeholder: "IP to send forwarded queries from — auto-assigned if blank (optional)"},
	{Name: "listening_address", Type: core.ConnectionTypeString, Label: "Listening Address", Placeholder: "IP to listen for queries on — auto-assigned if blank (optional)"},
	{Name: "nsg_ocids", Type: core.ConnectionTypeString, Label: "Network Security Group OCIDs", Placeholder: "Comma-separated NSG OCIDs in the same VCN (optional)"},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL or PRIVATE — resolver endpoints are private DNS", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resolver_endpoint", Type: core.ConnectionTypeObject, Label: "Resolver Endpoint"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Endpoint Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	resolverID, err := dnsn.RequiredString("resolver_ocid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	name, err := dnsn.RequiredString("endpoint_name", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	subnet, err := dnsn.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	isForwarding := dnsn.OptionalBool("is_forwarding", inputs, false)
	isListening := dnsn.OptionalBool("is_listening", inputs, false)
	// A resolver endpoint must forward, listen, or both — OCI rejects one that does
	// neither, so catch it up front with a clear message.
	if !isForwarding && !isListening {
		return dnsn.ErrorResult("a resolver endpoint must forward, listen, or both — enable Is Forwarding and/or Is Listening"), nil
	}
	details := dns.CreateResolverVnicEndpointDetails{
		Name:         &name,
		IsForwarding: &isForwarding,
		IsListening:  &isListening,
		SubnetId:     &subnet,
	}
	if v := strings.TrimSpace(dnsn.OptionalString("forwarding_address", inputs)); v != "" {
		details.ForwardingAddress = &v
	}
	if v := strings.TrimSpace(dnsn.OptionalString("listening_address", inputs)); v != "" {
		details.ListeningAddress = &v
	}
	if nsgs := dnsn.InputStrings("nsg_ocids", inputs); len(nsgs) > 0 {
		details.NsgIds = nsgs
	}
	req := dns.CreateResolverEndpointRequest{ResolverId: &resolverID, CreateResolverEndpointDetails: details}
	if scope != "" {
		req.Scope = dns.CreateResolverEndpointScopeEnum(scope)
	}
	resp, err := client.CreateResolverEndpoint(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	endpoint := dnsn.SummariseResolverEndpoint(resp.ResolverEndpoint)
	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Creating resolver endpoint %q (%s) — poll Get Resolver Endpoint until ACTIVE", name, endpoint["lifecycle_state"]),
		"resolver_endpoint": endpoint,
		"name":              endpoint["name"],
		"work_request_id":   dnsn.Str(resp.OpcWorkRequestId),
		"success":           true,
	}, nil
}
