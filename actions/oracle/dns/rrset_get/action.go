// Package oracle_dns_rrset_get reads the records of a specific RRSet — all records at a
// given domain of a given type (e.g. the A records for www.example.com).
package oracle_dns_rrset_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Get RRSet"
	Description  = "Read all records of one RRSet — the records at a given domain of a given type, e.g. the A records for www.example.com."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the zone picker)"},
	{Name: "zone_name_or_ocid", Type: core.ConnectionTypeString, Label: "Zone Name or OCID", Placeholder: "The zone FQDN or its OCID", Required: true},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "The fully-qualified record name, e.g. www.example.com", Required: true},
	{Name: "rtype", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "A, AAAA, CNAME, MX, TXT, NS, SRV, …", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL or PRIVATE (optional)", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "records", Type: core.ConnectionTypeObject, Label: "Records"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, zone, errResult := dnsn.ResourceClient(inputs, "zone_name_or_ocid")
	if errResult != nil {
		return errResult, nil
	}
	domain, err := dnsn.RequiredString("domain", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	rtype, err := dnsn.RequiredString("rtype", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.GetRRSetRequest{ZoneNameOrId: &zone, Domain: &domain, Rtype: &rtype}
	if scope != "" {
		req.Scope = dns.GetRRSetScopeEnum(scope)
	}
	// A large RRSet (e.g. a big round-robin set) paginates via OpcNextPage — walk it,
	// bounded by the shared page cap, and flag when the cap is hit.
	var items []dns.Record
	truncated := false
	for page := 0; ; page++ {
		if page >= dnsn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.GetRRSet(dnsn.Context(), req)
		if err != nil {
			return dnsn.ErrorResult(auth.OCIError(err)), nil
		}
		items = append(items, resp.Items...)
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	records := dnsn.SummariseRecords(items)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("RRSet %s/%s has %d record(s)", domain, rtype, len(records)),
		"records":     records,
		"count":       fmt.Sprintf("%d", len(records)),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
