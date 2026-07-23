// Package oracle_dns_zone_create creates a DNS zone (public GLOBAL or private) in a
// compartment. Zone provisioning is asynchronous — the call returns the zone with its
// OCID immediately (in a CREATING state) plus a work-request id; poll Get Zone until
// it is ACTIVE.
package oracle_dns_zone_create

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Create Zone"
	Description  = "Create an Oracle Cloud DNS zone in a compartment — a public (GLOBAL) or private zone. Returns the zone's OCID immediately plus a work-request id; poll Get Zone until it is ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+globe"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "zone_name", Type: core.ConnectionTypeString, Label: "Zone Name", Placeholder: "The fully-qualified zone name, e.g. example.com", Required: true},
	{Name: "zone_type", Type: core.ConnectionTypeString, Label: "Zone Type", Placeholder: "PRIMARY (default) or SECONDARY", Options: []core.ConnectionOption{
		{Name: "Primary", Value: "PRIMARY"},
		{Name: "Secondary", Value: "SECONDARY"},
	}},
	{Name: "external_masters_json", Type: core.ConnectionTypeText, Label: "External Masters (JSON array)", Placeholder: `[{"address":"192.0.2.1","port":53,"tsigKeyId":"ocid1.dnstsigkey.oc1..aaaa…"}] — REQUIRED for a SECONDARY zone (the masters it transfers from); ignored for PRIMARY`},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public, default) or PRIVATE", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
	{Name: "view_ocid", Type: core.ConnectionTypeString, Label: "View OCID", Placeholder: "Required for a PRIVATE zone — the view it belongs to (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	name, err := dnsn.RequiredString("zone_name", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	details := dns.CreateZoneDetails{Name: &name, CompartmentId: &compartment}
	switch strings.ToUpper(strings.TrimSpace(dnsn.OptionalString("zone_type", inputs))) {
	case "SECONDARY":
		details.ZoneType = dns.CreateZoneDetailsZoneTypeSecondary
	case "", "PRIMARY":
		details.ZoneType = dns.CreateZoneDetailsZoneTypePrimary
	default:
		return dnsn.ErrorResult("zone type must be PRIMARY or SECONDARY"), nil
	}
	// A SECONDARY zone transfers from external master servers, so OCI requires at least
	// one — catch a missing/empty set up front rather than surfacing an opaque 400.
	if details.ZoneType == dns.CreateZoneDetailsZoneTypeSecondary {
		var masters []dns.ExternalMaster
		if raw := strings.TrimSpace(dnsn.OptionalString("external_masters_json", inputs)); raw != "" {
			if err := json.Unmarshal([]byte(raw), &masters); err != nil {
				return dnsn.ErrorResult(fmt.Sprintf(`external masters must be a JSON array of {address, port, tsigKeyId} objects, e.g. [{"address":"192.0.2.1","port":53}]: %s`, err.Error())), nil
			}
		}
		if len(masters) == 0 {
			return dnsn.ErrorResult(`a SECONDARY zone requires at least one external master — supply external_masters_json, e.g. [{"address":"192.0.2.1","port":53}]`), nil
		}
		details.ExternalMasters = masters
	}
	if scope != "" {
		details.Scope = dns.ScopeEnum(scope)
	}
	if v := strings.TrimSpace(dnsn.OptionalString("view_ocid", inputs)); v != "" {
		details.ViewId = &v
	}
	if tags, err := dnsn.FreeformTags("tags", inputs); err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	req := dns.CreateZoneRequest{CreateZoneDetails: details}
	if scope != "" {
		req.Scope = dns.CreateZoneScopeEnum(scope)
	}
	resp, err := client.CreateZone(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	zone := dnsn.SummariseZone(&resp.Zone)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Creating zone %q (%s) — poll Get Zone until ACTIVE", name, zone["lifecycle_state"]),
		"zone":            zone,
		"id":              zone["id"],
		"lifecycle_state": zone["lifecycle_state"],
		"work_request_id": dnsn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
