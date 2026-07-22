// Package oracle_dns_zone_get_content exports a DNS zone's records as BIND zone-file text.
package oracle_dns_zone_get_content

import (
	"fmt"
	"io"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Get Zone Content"
	Description  = "Export an Oracle Cloud DNS zone as BIND zone-file text — the full set of records for the zone, ready to inspect or migrate."
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
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private zones", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
	{Name: "view_ocid", Type: core.ConnectionTypeString, Label: "View OCID", Placeholder: "ocid1.dns-view.oc1..aaaa… — required when accessing a private zone by name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Zone File (BIND)"},
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
	req := dns.GetZoneContentRequest{ZoneNameOrId: &id}
	if scope != "" {
		req.Scope = dns.GetZoneContentScopeEnum(scope)
	}
	if view := dnsn.OptionalString("view_ocid", inputs); view != "" {
		req.ViewId = &view
	}
	resp, err := client.GetZoneContent(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	if resp.Content == nil {
		return dnsn.ErrorResult("OCI returned no zone-file content"), nil
	}
	defer resp.Content.Close()
	body, err := io.ReadAll(resp.Content)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	content := string(body)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Exported zone %q (%d bytes of BIND zone-file text)", id, len(body)),
		"content":     content,
		"success":     true,
	}, nil
}
