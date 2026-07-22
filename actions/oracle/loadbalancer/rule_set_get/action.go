// Package oracle_loadbalancer_rule_set_get reads one rule set of a load balancer
// by name.
package oracle_loadbalancer_rule_set_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Get Rule Set"
	Description  = "Fetch one rule set of an Oracle Cloud load balancer by name — its request/response control rules."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code-branch"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "rule_set_name", Type: core.ConnectionTypeString, Label: "Rule Set Name", Placeholder: "The rule set name, e.g. example_rule_set", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_set", Type: core.ConnectionTypeObject, Label: "Rule Set"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Rule Set Name"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := lbn.RequiredString("rule_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetRuleSet(lbn.Context(), lb.GetRuleSetRequest{LoadBalancerId: &lbID, RuleSetName: &name})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	summary := lbn.SummariseRuleSet(&resp.RuleSet)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Rule set %q", summary["name"]),
		"rule_set":    summary,
		"name":        summary["name"],
		"success":     true,
	}, nil
}
