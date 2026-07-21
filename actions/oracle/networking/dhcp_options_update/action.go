// Package oracle_networking_dhcp_options_update updates the editable attributes of a
// set of DHCP options — its display name, domain-name type, the option entries (DNS
// servers / search domains) and freeform tags.
package oracle_networking_dhcp_options_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Update DHCP Options"
	Description  = "Update editable attributes of an Oracle Cloud DHCP options set — display name, domain-name type, and the option entries (supply Options JSON to replace the DNS-server / search-domain entries)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "dhcp_options_ocid", Type: core.ConnectionTypeString, Label: "DHCP Options OCID", Placeholder: "ocid1.dhcpoptions.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "domain_name_type", Type: core.ConnectionTypeString, Label: "Domain Name Type", Placeholder: "SUBNET_DOMAIN, VCN_DOMAIN or CUSTOM_DOMAIN (optional)"},
	{Name: "options_json", Type: core.ConnectionTypeText, Label: "DHCP Options (JSON)", Placeholder: `[{"type":"DomainNameServer","serverType":"VcnLocalPlusInternet"}] — replaces all option entries when supplied (optional)`},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces all freeform tags when supplied (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dhcp_options", Type: core.ConnectionTypeObject, Label: "DHCP Options"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "dhcp_options_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	// REPLACE semantics: the option entries are only touched when the operator
	// supplies JSON (an empty input leaves the existing DNS/search-domain config).
	options, err := net.DecodeDhcpOptions("options_json", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateDhcpDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("domain_name_type", inputs)); v != "" {
		details.DomainNameType = ocicore.UpdateDhcpDetailsDomainNameTypeEnum(v)
	}
	if len(options) > 0 {
		details.Options = options
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateDhcpOptions(net.Context(), ocicore.UpdateDhcpOptionsRequest{
		DhcpId:            &id,
		UpdateDhcpDetails: details,
	})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	opts := net.SummariseDhcpOptions(&resp.DhcpOptions)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated DHCP options %q (%s)", opts["display_name"], opts["lifecycle_state"]),
		"dhcp_options":    opts,
		"lifecycle_state": opts["lifecycle_state"],
		"success":         true,
	}, nil
}
