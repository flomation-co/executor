// Package oracle_dns_zone_create_from_file creates a DNS zone by importing a raw
// BIND-format zone file, provisioning the zone plus every record it declares in one call.
package oracle_dns_zone_create_from_file

import (
	"fmt"
	"io"
	"strings"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Create Zone from Zone File"
	Description  = "Create an Oracle Cloud DNS zone by importing a raw BIND-format zone file — provisioning the zone and every record it declares in one call. Returns the zone's OCID immediately plus a work-request id; poll Get Zone until it is ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+globe"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "zone_file", Type: core.ConnectionTypeText, Label: "Zone File (BIND format)", Placeholder: "The full BIND-format zone file text, e.g. $ORIGIN example.com.\n@ IN SOA …", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public, default) or PRIVATE", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
	{Name: "view_ocid", Type: core.ConnectionTypeString, Label: "View OCID", Placeholder: "Required for a PRIVATE zone — the view it belongs to (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "zone", Type: core.ConnectionTypeObject, Label: "Zone"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Zone OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	zoneFile, err := dnsn.RequiredString("zone_file", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.CreateZoneFromZoneFileRequest{
		CompartmentId:                 &compartment,
		CreateZoneFromZoneFileDetails: io.NopCloser(strings.NewReader(zoneFile)),
	}
	if scope != "" {
		req.Scope = dns.CreateZoneFromZoneFileScopeEnum(scope)
	}
	if v := strings.TrimSpace(dnsn.OptionalString("view_ocid", inputs)); v != "" {
		req.ViewId = &v
	}
	resp, err := client.CreateZoneFromZoneFile(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	zone := dnsn.SummariseZone(&resp.Zone)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Importing zone %q (%s) from zone file — poll Get Zone until ACTIVE", zone["name"], zone["lifecycle_state"]),
		"zone":            zone,
		"id":              zone["id"],
		"lifecycle_state": zone["lifecycle_state"],
		"work_request_id": dnsn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
