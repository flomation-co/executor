// Package oracle_loadbalancer_rule_set_update replaces the rules that compose a rule
// set on an Oracle Cloud load balancer. Asynchronous — returns a work-request id.
package oracle_loadbalancer_rule_set_update

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Update Rule Set"
	Description  = "Update a rule set on an Oracle Cloud load balancer — replaces the whole array of rules that compose the set. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code-branch"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "rule_set_name", Type: core.ConnectionTypeString, Label: "Rule Set Name", Placeholder: "The name of the rule set to update, e.g. example_rule_set", Required: true},
	{Name: "items_json", Type: core.ConnectionTypeString, Label: "Rules (JSON)", Placeholder: "A JSON array of rule objects, each keyed by \"action\", e.g. [{\"action\":\"ADD_HTTP_REQUEST_HEADER\",\"header\":\"X-Env\",\"value\":\"prod\"}]", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_set_name", Type: core.ConnectionTypeString, Label: "Rule Set Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	itemsJSON, err := lbn.RequiredString("items_json", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// Polymorphic body: the rule set's items are a JSON array of typed rule objects,
	// each discriminated by its "action" field. Decode into the SDK's Rule slice; a
	// malformed array is surfaced with a helpful message.
	var items []lb.Rule
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return lbn.ErrorResult("items_json must be a JSON array of rule objects, e.g. [{\"action\":\"ADD_HTTP_REQUEST_HEADER\",\"header\":\"X-Env\",\"value\":\"prod\"}]: " + err.Error()), nil
	}
	// Replace-semantics: UpdateRuleSetDetails overwrites the entire set of rules, so the
	// items supplied here become the rule set in full — exactly as rule_set_create builds it.
	details := lb.UpdateRuleSetDetails{Items: items}
	resp, err := client.UpdateRuleSet(lbn.Context(), lb.UpdateRuleSetRequest{
		LoadBalancerId:       &lbID,
		RuleSetName:          &name,
		UpdateRuleSetDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updating rule set %q (%d rule(s)) on load balancer %s — poll work request %s", name, len(items), lbID, lbn.Str(resp.OpcWorkRequestId)),
		"rule_set_name":   name,
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
