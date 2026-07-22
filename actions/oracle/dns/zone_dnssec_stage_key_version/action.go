// Package oracle_dns_zone_dnssec_stage_key_version stages a new successor DNSSEC key
// version on a zone. Asynchronous — returns a work-request id.
package oracle_dns_zone_dnssec_stage_key_version

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Stage Zone DNSSEC Key Version"
	Description  = "Stage a new successor DNSSEC key version on an Oracle Cloud DNS zone, given the UUID of the key version it succeeds. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+lock"
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
	{Name: "zone_ocid", Type: core.ConnectionTypeString, Label: "Zone OCID", Placeholder: "ocid1.dns-zone.oc1..aaaa… (the OCID of the target zone)", Required: true},
	{Name: "predecessor_key_version_uuid", Type: core.ConnectionTypeString, Label: "Predecessor Key Version UUID", Placeholder: "UUID of the DNSSEC key version to generate a successor for", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private zones", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Zone OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, zoneID, errResult := dnsn.ResourceClient(inputs, "zone_ocid")
	if errResult != nil {
		return errResult, nil
	}
	predecessor, err := dnsn.RequiredString("predecessor_key_version_uuid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.StageZoneDnssecKeyVersionRequest{
		ZoneId: &zoneID,
		StageZoneDnssecKeyVersionDetails: dns.StageZoneDnssecKeyVersionDetails{
			PredecessorDnssecKeyVersionUuid: &predecessor,
		},
	}
	if scope != "" {
		req.Scope = dns.StageZoneDnssecKeyVersionScopeEnum(scope)
	}
	resp, err := client.StageZoneDnssecKeyVersion(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	result := dnsn.AsyncResult(
		fmt.Sprintf("Staged successor DNSSEC key version for %s on zone %s", predecessor, zoneID),
		dnsn.Str(resp.OpcWorkRequestId),
	)
	result["id"] = zoneID
	return result, nil
}
