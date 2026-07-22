// Package oracle_dns_zone_records_patch applies a set of add/remove record operations
// (with optional preconditions) to a zone's records in one atomic PATCH — the surgical
// alternative to replacing a whole RRSet.
package oracle_dns_zone_records_patch

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
	Name         = "OCI DNS: Patch Zone Records"
	Description  = "Apply a set of record operations (ADD/REMOVE, with optional REQUIRE/PROHIBIT preconditions) to a zone's records in one atomic PATCH — the surgical alternative to replacing a whole RRSet. Synchronous."
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
	{Name: "items_json", Type: core.ConnectionTypeText, Label: "Operations (JSON array)", Placeholder: `[{"domain":"www.example.com","rtype":"A","rdata":"10.0.0.5","ttl":300,"operation":"ADD"},{"domain":"old.example.com","rtype":"A","rdata":"10.0.0.4","operation":"REMOVE"}] — each item: domain, rtype, rdata, ttl, recordHash, operation (ADD/REMOVE/REQUIRE/PROHIBIT)`, Required: true},
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
	var items []dns.RecordOperation
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return dnsn.ErrorResult(fmt.Sprintf(`operations must be a JSON array of record-operation objects, e.g. [{"domain":"www.example.com","rtype":"A","rdata":"10.0.0.5","ttl":300,"operation":"ADD"}]: %s`, err.Error())), nil
	}
	if len(items) == 0 {
		return dnsn.ErrorResult("at least one record operation is required"), nil
	}
	req := dns.PatchZoneRecordsRequest{
		ZoneNameOrId:            &zone,
		PatchZoneRecordsDetails: dns.PatchZoneRecordsDetails{Items: items},
	}
	if scope != "" {
		req.Scope = dns.PatchZoneRecordsScopeEnum(scope)
	}
	resp, err := client.PatchZoneRecords(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	records := dnsn.SummariseRecords(resp.Items)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Patched zone %q; it now holds %d record(s)", zone, len(records)),
		"records":     records,
		"count":       fmt.Sprintf("%d", len(records)),
		"success":     true,
	}, nil
}
