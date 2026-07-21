// Package oracle_networking_nsg_rule_add adds security rules to a Network Security
// Group. NSG rules live in a separate sub-API keyed on the NSG OCID; OCI assigns
// each new rule an id (returned here) used to update or remove it later.
package oracle_networking_nsg_rule_add

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Add NSG Rules"
	Description  = "Add security rules to a Network Security Group. Supply the rules as a JSON array, e.g. [{\"direction\":\"INGRESS\",\"protocol\":\"6\",\"source\":\"0.0.0.0/0\",\"tcpOptions\":{\"destinationPortRange\":{\"min\":22,\"max\":22}}}]. OCI returns each new rule's id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "security_rules", Type: core.ConnectionTypeText, Label: "Security Rules (JSON)", Placeholder: `[{"direction":"INGRESS","protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":22,"max":22}}}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_rules", Type: core.ConnectionTypeObject, Label: "Added Rules"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "nsg_ocid")
	if errResult != nil {
		return errResult, nil
	}
	rules, err := net.DecodeNsgAddRules("security_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if len(rules) == 0 {
		return net.ErrorResult("at least one security rule is required (each with a direction and protocol)"), nil
	}
	resp, err := client.AddNetworkSecurityGroupSecurityRules(net.Context(), ocicore.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId:                      &id,
		AddNetworkSecurityGroupSecurityRulesDetails: ocicore.AddNetworkSecurityGroupSecurityRulesDetails{SecurityRules: rules},
	})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Added %d rule(s) to NSG %s", len(resp.SecurityRules), id),
		"security_rules": resp.SecurityRules,
		"count":          len(resp.SecurityRules),
		"success":        true,
	}, nil
}
