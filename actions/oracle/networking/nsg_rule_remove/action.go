// Package oracle_networking_nsg_rule_remove removes security rules from a Network
// Security Group by their rule ids. NSG rules live in a separate sub-API keyed on
// the NSG OCID; each rule's id is assigned by OCI when the rule is added.
package oracle_networking_nsg_rule_remove

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Remove NSG Rules"
	Description  = "Remove security rules from a Network Security Group by their rule ids."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+trash"
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
	{Name: "security_rule_ids", Type: core.ConnectionTypeString, Label: "Security Rule IDs", Placeholder: "Comma-separated rule ids to remove, e.g. rule-a,rule-b", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "nsg_ocid")
	if errResult != nil {
		return errResult, nil
	}
	ids := net.InputStrings("security_rule_ids", inputs)
	if len(ids) == 0 {
		return net.ErrorResult("at least one security rule id is required"), nil
	}
	_, err := client.RemoveNetworkSecurityGroupSecurityRules(net.Context(), ocicore.RemoveNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId:                         &id,
		RemoveNetworkSecurityGroupSecurityRulesDetails: ocicore.RemoveNetworkSecurityGroupSecurityRulesDetails{SecurityRuleIds: ids},
	})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Removed %d rule(s) from NSG %s", len(ids), id),
		"success":     true,
	}, nil
}
