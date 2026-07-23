// Package oracle_networking_nsg_rule_get_all lists the security rules in an
// Oracle Cloud Network Security Group. NSG rules live in a separate sub-API
// keyed on the NSG OCID; each rule carries the id used to update or remove it.
package oracle_networking_nsg_rule_get_all

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
	Name         = "OCI Networking: List NSG Rules"
	Description  = "List the security rules in a Network Security Group."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
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
	{Name: "nsg_ocid", Type: core.ConnectionTypeString, Label: "NSG OCID", Placeholder: "ocid1.networksecuritygroup.oc1..aaaa…", Required: true},
	{Name: "direction", Type: core.ConnectionTypeString, Label: "Direction filter", Placeholder: "Only rules in this direction (optional)", Options: []core.ConnectionOption{
		{Name: "Any (both directions)", Value: ""},
		{Name: "Ingress (inbound)", Value: "INGRESS"},
		{Name: "Egress (outbound)", Value: "EGRESS"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_rules", Type: core.ConnectionTypeObject, Label: "Security Rules"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "nsg_ocid")
	if errResult != nil {
		return errResult, nil
	}
	ctx := net.Context()

	req := ocicore.ListNetworkSecurityGroupSecurityRulesRequest{NetworkSecurityGroupId: &id}
	if v := strings.TrimSpace(net.OptionalString("direction", inputs)); v != "" {
		req.Direction = ocicore.ListNetworkSecurityGroupSecurityRulesDirectionEnum(v)
	}
	var items []ocicore.SecurityRule
	truncated := false
	for page := 0; page < net.ListMaxPages; page++ {
		resp, err := client.ListNetworkSecurityGroupSecurityRules(ctx, req)
		if err != nil {
			return net.ErrorResult(auth.OCIError(err)), nil
		}
		items = append(items, resp.Items...)
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == net.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d security rule(s) in NSG %s", len(items), id)
	if truncated {
		summary = fmt.Sprintf("Found at least %d security rule(s) in NSG %s (list truncated at %d pages — more available)", len(items), id, net.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result":    summary,
		"security_rules": items,
		"count":          len(items),
		"truncated":      truncated,
		"success":        true,
	}, nil
}
