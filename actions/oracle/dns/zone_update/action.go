// Package oracle_dns_zone_update updates a DNS zone's freeform tags and external master
// servers, preserving everything else the zone currently carries.
package oracle_dns_zone_update

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
	Name         = "OCI DNS: Update Zone"
	Description  = "Update a DNS zone's external master servers and freeform tags. Reads the zone first and re-sends its current external masters/tags/config, overwriting only the fields you supply — a SECONDARY zone's external masters are preserved when left blank. Returns a work-request id."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the zone picker)"},
	{Name: "zone_name_or_ocid", Type: core.ConnectionTypeString, Label: "Zone Name or OCID", Placeholder: "The zone FQDN (e.g. example.com) or its OCID", Required: true},
	{Name: "external_masters_json", Type: core.ConnectionTypeText, Label: "External Masters (JSON array)", Placeholder: `[{"address":"192.0.2.1","port":53,"tsigKeyId":"ocid1.dnstsigkey.oc1..aaaa…"}] — REPLACES the zone's external masters; leave blank to keep the current ones`},
	{Name: "tags", Type: core.ConnectionTypeText, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — REPLACES the zone's freeform tags; leave blank to keep the current ones`},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private zones", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "zone", Type: core.ConnectionTypeObject, Label: "Zone"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Zone OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "zone_name_or_ocid")
	if errResult != nil {
		return errResult, nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}

	// READ: fetch the zone so we can re-send its current external masters/tags/config
	// rather than wiping them (UpdateZone is a full-replace PUT).
	getReq := dns.GetZoneRequest{ZoneNameOrId: &id}
	if scope != "" {
		getReq.Scope = dns.GetZoneScopeEnum(scope)
	}
	getResp, err := client.GetZone(dnsn.Context(), getReq)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	current := getResp.Zone

	// MODIFY: seed the update body from the current values.
	details := dns.UpdateZoneDetails{
		FreeformTags:        current.FreeformTags,
		DefinedTags:         current.DefinedTags,
		ResolutionMode:      dns.ZoneResolutionModeEnum(current.ResolutionMode),
		DnssecState:         dns.ZoneDnssecStateEnum(current.DnssecState),
		ExternalMasters:     current.ExternalMasters,
		ExternalDownstreams: current.ExternalDownstreams,
	}

	// Overlay only the operator-supplied inputs.
	if raw := strings.TrimSpace(dnsn.OptionalString("external_masters_json", inputs)); raw != "" {
		var masters []dns.ExternalMaster
		if err := json.Unmarshal([]byte(raw), &masters); err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`external masters must be a JSON array of {address, port, tsigKeyId} objects, e.g. [{"address":"192.0.2.1","port":53}]: %s`, err.Error())), nil
		}
		details.ExternalMasters = masters
	}
	if dnsn.BoolWasSet("tags", inputs) {
		tags, err := dnsn.FreeformTags("tags", inputs)
		if err != nil {
			return dnsn.ErrorResult(err.Error()), nil
		}
		details.FreeformTags = tags
	}

	// WRITE.
	req := dns.UpdateZoneRequest{ZoneNameOrId: &id, UpdateZoneDetails: details}
	if scope != "" {
		req.Scope = dns.UpdateZoneScopeEnum(scope)
	}
	resp, err := client.UpdateZone(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	zone := dnsn.SummariseZone(&resp.Zone)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated zone %q", zone["name"]),
		"zone":            zone,
		"id":              zone["id"],
		"work_request_id": dnsn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
