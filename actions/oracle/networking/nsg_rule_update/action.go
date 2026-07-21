// Package oracle_networking_nsg_rule_update updates existing security rules in a
// Network Security Group. NSG rules live in a separate sub-API keyed on the NSG
// OCID; each rule supplied here must carry the id OCI assigned it when it was added.
package oracle_networking_nsg_rule_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Update NSG Rules"
	Description  = "Update existing security rules in a Network Security Group. Each rule in the JSON array must include its id."
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
	{Name: "nsg_ocid", Type: core.ConnectionTypeString, Label: "NSG OCID", Placeholder: "ocid1.networksecuritygroup.oc1..aaaa…", Required: true},
	{Name: "security_rules", Type: core.ConnectionTypeText, Label: "Security Rules (JSON)", Placeholder: `[{"id":"…","direction":"INGRESS","protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":22,"max":22}}}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_rules", Type: core.ConnectionTypeObject, Label: "Updated Rules"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "nsg_ocid")
	if errResult != nil {
		return errResult, nil
	}
	rules, err := net.DecodeNsgUpdateRules("security_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if len(rules) == 0 {
		return net.ErrorResult("at least one security rule is required (each with its id)"), nil
	}
	resp, err := client.UpdateNetworkSecurityGroupSecurityRules(net.Context(), ocicore.UpdateNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId:                         &id,
		UpdateNetworkSecurityGroupSecurityRulesDetails: ocicore.UpdateNetworkSecurityGroupSecurityRulesDetails{SecurityRules: rules},
	})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Updated %d rule(s) in NSG %s", len(resp.SecurityRules), id),
		"security_rules": resp.SecurityRules,
		"count":          len(resp.SecurityRules),
		"success":        true,
	}, nil
}
