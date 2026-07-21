// Package oracle_networking_public_ip_get_all lists the public IPs in an Oracle
// Cloud compartment for a given scope (regional or availability-domain).
package oracle_networking_public_ip_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: List Public IPs"
	Description  = "List public IPs in a compartment. Requires a scope (REGION or AVAILABILITY_DOMAIN)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
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
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "Whether the public IP is regional or availability-domain specific", Required: true, Options: []core.ConnectionOption{
		{Name: "Region", Value: "REGION"},
		{Name: "Availability Domain", Value: "AVAILABILITY_DOMAIN"},
	}},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 — required when the scope is AVAILABILITY_DOMAIN", Visible: &core.VisibleWhen{Field: "scope", Values: []string{"AVAILABILITY_DOMAIN"}}},
	{Name: "lifetime", Type: core.ConnectionTypeString, Label: "Lifetime Filter", Placeholder: "Only public IPs of this lifetime (optional)", Options: []core.ConnectionOption{
		{Name: "Ephemeral", Value: "EPHEMERAL"},
		{Name: "Reserved", Value: "RESERVED"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "public_ips", Type: core.ConnectionTypeObject, Label: "Public IPs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := net.GetAuth(inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}

	scope := strings.TrimSpace(net.OptionalString("scope", inputs))
	if scope == "" {
		scope = string(ocicore.ListPublicIpsScopeRegion)
	}
	req := ocicore.ListPublicIpsRequest{
		Scope:         ocicore.ListPublicIpsScopeEnum(scope),
		CompartmentId: &compartment,
	}
	if ad := strings.TrimSpace(net.OptionalString("availability_domain", inputs)); ad != "" {
		req.AvailabilityDomain = &ad
	} else if strings.EqualFold(scope, string(ocicore.ListPublicIpsScopeAvailabilityDomain)) {
		return net.ErrorResult("an availability domain is required when the scope is AVAILABILITY_DOMAIN (e.g. Uocm:UK-LONDON-1-AD-1)"), nil
	}
	if lifetime := strings.TrimSpace(net.OptionalString("lifetime", inputs)); lifetime != "" {
		req.Lifetime = ocicore.ListPublicIpsLifetimeEnum(lifetime)
	}

	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := net.Context()

	var items []map[string]interface{}
	truncated := false
	for page := 0; page < net.ListMaxPages; page++ {
		resp, err := client.ListPublicIps(ctx, req)
		if err != nil {
			return net.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			items = append(items, net.SummarisePublicIp(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == net.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d public IP(s) in the compartment", len(items))
	if truncated {
		summary = fmt.Sprintf("Found at least %d public IP(s) (list truncated at %d pages — more available)", len(items), net.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"public_ips":  items,
		"count":       len(items),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
