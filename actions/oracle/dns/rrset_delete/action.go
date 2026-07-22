// Package oracle_dns_rrset_delete deletes an entire RRSet (all records of one type
// at one domain) within a DNS zone. Synchronous — no work-request id.
package oracle_dns_rrset_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Delete RRSet"
	Description  = "Delete every record of one type (RRSet) at a domain within an Oracle Cloud DNS zone. Synchronous — the RRSet is gone once it returns."
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
	{Name: "zone_name_or_ocid", Type: core.ConnectionTypeString, Label: "Zone Name or OCID", Placeholder: "The zone FQDN (e.g. example.com) or its OCID", Required: true},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "The FQDN within the zone, e.g. www.example.com", Required: true},
	{Name: "rtype", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "e.g. A, AAAA, CNAME, MX, TXT", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private zones", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Zone (name or OCID)"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain"},
	{Name: "rtype", Type: core.ConnectionTypeString, Label: "Record Type"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "zone_name_or_ocid")
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
	req := dns.DeleteRRSetRequest{ZoneNameOrId: &id, Domain: &domain, Rtype: &rtype}
	if scope != "" {
		req.Scope = dns.DeleteRRSetScopeEnum(scope)
	}
	if _, err := client.DeleteRRSet(dnsn.Context(), req); err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted the %s RRSet at %s in zone %s", rtype, domain, id),
		"id":          id,
		"domain":      domain,
		"rtype":       rtype,
		"success":     true,
	}, nil
}
