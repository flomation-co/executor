// Package oracle_dns_domain_records_update replaces EVERY record at one domain of a
// zone — across all record types — with a supplied set. This is the domain-wide
// counterpart to Update RRSet: where RRSet update touches only one type, this rewrites
// the whole domain, so any type not present in the supplied set is removed.
package oracle_dns_domain_records_update

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Update Domain Records"
	Description  = "Replace ALL records at one domain of a zone — every record type — with a supplied set. This rewrites the whole domain: any record type not present in the supplied set is deleted. Synchronous."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the zone picker)"},
	{Name: "zone_name_or_ocid", Type: core.ConnectionTypeString, Label: "Zone Name or OCID", Placeholder: "The zone FQDN or its OCID", Required: true},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "The fully-qualified record name, e.g. www.example.com", Required: true},
	{Name: "records_json", Type: core.ConnectionTypeText, Label: "Records (JSON array)", Placeholder: `[{"rtype":"A","rdata":"10.0.0.5","ttl":300}] — rtype + rdata + ttl per record; domain defaults to the one above (REPLACES EVERY record at the domain, all types)`, Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL or PRIVATE (optional)", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "records", Type: core.ConnectionTypeObject, Label: "Records"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
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
	raw, err := dnsn.RequiredString("records_json", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	var items []dns.RecordDetails
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return dnsn.ErrorResult(fmt.Sprintf(`records must be a JSON array of {rtype, rdata, ttl} objects, e.g. [{"rtype":"A","rdata":"10.0.0.5","ttl":300}]: %s`, err.Error())), nil
	}
	if len(items) == 0 {
		return dnsn.ErrorResult("at least one record is required — an empty set would delete every record at the domain (use the dedicated delete action if that is the intent)"), nil
	}
	// Default each record's domain to the target domain so callers only supply rtype/rdata/ttl.
	for i := range items {
		if items[i].Domain == nil {
			d := domain
			items[i].Domain = &d
		}
	}
	req := dns.UpdateDomainRecordsRequest{
		ZoneNameOrId:               &zone,
		Domain:                     &domain,
		UpdateDomainRecordsDetails: dns.UpdateDomainRecordsDetails{Items: items},
	}
	if scope != "" {
		req.Scope = dns.UpdateDomainRecordsScopeEnum(scope)
	}
	resp, err := client.UpdateDomainRecords(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	records := dnsn.SummariseRecords(resp.Items)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Domain %s now has %d record(s) across all types", domain, len(records)),
		"records":     records,
		"count":       fmt.Sprintf("%d", len(records)),
		"success":     true,
	}, nil
}
