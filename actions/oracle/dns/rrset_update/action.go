// Package oracle_dns_rrset_update replaces all records of one RRSet (the records at a
// domain of a given type) with a supplied set. This is the primary way to set a
// record — e.g. point www.example.com at a new IP.
package oracle_dns_rrset_update

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
	Name         = "OCI DNS: Update RRSet"
	Description  = "Replace all records of one RRSet (the records at a domain of a given type) with a supplied set — the primary way to set a record, e.g. point www.example.com at a new IP. Synchronous."
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
	{Name: "rtype", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "A, AAAA, CNAME, MX, TXT, NS, …", Required: true},
	{Name: "records_json", Type: core.ConnectionTypeText, Label: "Records (JSON array)", Placeholder: `[{"rdata":"10.0.0.5","ttl":300}] — rdata + ttl; domain/rtype default to the ones above (REPLACES the whole RRSet)`, Required: true},
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
	rtype, err := dnsn.RequiredString("rtype", inputs)
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
		return dnsn.ErrorResult(fmt.Sprintf(`records must be a JSON array of {rdata, ttl} objects, e.g. [{"rdata":"10.0.0.5","ttl":300}]: %s`, err.Error())), nil
	}
	if len(items) == 0 {
		return dnsn.ErrorResult("at least one record is required — an empty set would delete the RRSet (use the dedicated delete action if that is the intent)"), nil
	}
	// Default each record's domain/rtype to the RRSet's own so callers only supply rdata/ttl.
	for i := range items {
		if items[i].Domain == nil {
			d := domain
			items[i].Domain = &d
		}
		if items[i].Rtype == nil {
			t := rtype
			items[i].Rtype = &t
		}
	}
	req := dns.UpdateRRSetRequest{
		ZoneNameOrId:       &zone,
		Domain:             &domain,
		Rtype:              &rtype,
		UpdateRrSetDetails: dns.UpdateRrSetDetails{Items: items},
	}
	if scope != "" {
		req.Scope = dns.UpdateRRSetScopeEnum(scope)
	}
	resp, err := client.UpdateRRSet(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	records := dnsn.SummariseRecords(resp.Items)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Set %s/%s to %d record(s)", domain, rtype, len(records)),
		"records":     records,
		"count":       fmt.Sprintf("%d", len(records)),
		"success":     true,
	}, nil
}
