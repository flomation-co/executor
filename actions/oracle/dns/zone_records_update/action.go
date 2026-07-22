// Package oracle_dns_zone_records_update replaces EVERY record in a DNS zone with a
// supplied set — a whole-zone overwrite, not a per-RRSet edit. Whatever is in the zone
// today is discarded; the zone ends up holding exactly the records provided.
package oracle_dns_zone_records_update

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
	Name         = "OCI DNS: Update Zone Records"
	Description   = "Replace EVERY record in a DNS zone with a supplied set. This is a whole-zone overwrite: any record not in the list you provide is deleted, and the zone ends up holding exactly the records you supply. Synchronous."
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
	{Name: "items_json", Type: core.ConnectionTypeText, Label: "Records (JSON array)", Placeholder: `REPLACES THE WHOLE ZONE. Full record list, e.g. [{"domain":"www.example.com","rtype":"A","rdata":"10.0.0.5","ttl":300}] — every record needs domain, rtype, rdata and ttl. Anything omitted is deleted.`, Required: true},
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
	raw, err := dnsn.RequiredString("items_json", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	var items []dns.RecordDetails
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return dnsn.ErrorResult(fmt.Sprintf(`records must be a JSON array of {domain, rtype, rdata, ttl} objects, e.g. [{"domain":"www.example.com","rtype":"A","rdata":"10.0.0.5","ttl":300}]: %s`, err.Error())), nil
	}
	if len(items) == 0 {
		return dnsn.ErrorResult("at least one record is required — this replaces every record in the zone, and an empty list would wipe the whole zone"), nil
	}
	req := dns.UpdateZoneRecordsRequest{
		ZoneNameOrId:             &zone,
		UpdateZoneRecordsDetails: dns.UpdateZoneRecordsDetails{Items: items},
	}
	if scope != "" {
		req.Scope = dns.UpdateZoneRecordsScopeEnum(scope)
	}
	resp, err := client.UpdateZoneRecords(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	records := dnsn.SummariseRecords(resp.Items)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Zone %q now holds %d record(s)", zone, len(records)),
		"records":     records,
		"count":       fmt.Sprintf("%d", len(records)),
		"success":     true,
	}, nil
}
