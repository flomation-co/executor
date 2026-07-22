// Package oracle_dns_domain_records_delete deletes ALL records at a domain within a
// DNS zone. Synchronous (HTTP 204, no work request).
package oracle_dns_domain_records_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Delete Domain Records"
	Description  = "Delete ALL records at a domain (FQDN) within an Oracle Cloud DNS zone. Synchronous — every record type at that domain is removed."
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
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain (FQDN)", Placeholder: "The FQDN whose records to delete, e.g. www.example.com — ALL record types are removed", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private zones", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Zone (name or OCID)"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain (FQDN)"},
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
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.DeleteDomainRecordsRequest{ZoneNameOrId: &id, Domain: &domain}
	if scope != "" {
		req.Scope = dns.DeleteDomainRecordsScopeEnum(scope)
	}
	if _, err := client.DeleteDomainRecords(dnsn.Context(), req); err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted all records at %s in zone %s", domain, id),
		"id":          id,
		"domain":      domain,
		"success":     true,
	}, nil
}
