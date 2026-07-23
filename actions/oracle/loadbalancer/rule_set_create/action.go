// Package oracle_loadbalancer_rule_set_create adds a rule set — a named collection of
// HTTP header, redirect and access-control rules — to a load balancer. Asynchronous —
// returns a work-request id.
package oracle_loadbalancer_rule_set_create

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
	Name         = "OCI Load Balancer: Create Rule Set"
	Description  = "Add a rule set — a named set of HTTP header, redirect and access-control rules — to an Oracle Cloud load balancer. Asynchronous."
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
	{Name: "rule_set_name", Type: core.ConnectionTypeString, Label: "Rule Set Name", Placeholder: "A name unique within the load balancer, e.g. security-headers", Required: true},
	{Name: "items_json", Type: core.ConnectionTypeString, Label: "Rules (JSON)", Placeholder: `[{"action":"ADD_HTTP_REQUEST_HEADER","header":"X-Forwarded-Proto","value":"https"}] — each object needs an "action" (ADD_HTTP_REQUEST_HEADER, REDIRECT, ALLOW, CONTROL_ACCESS_USING_HTTP_METHODS, …)`, Required: true},
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
	rawItems, err := lbn.RequiredString("items_json", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// Items is a POLYMORPHIC []lb.Rule (an empty-interface slice keyed by each rule's
	// "action") — a bare json.Unmarshal into []lb.Rule can't dispatch to the concrete
	// rule structs, so we lean on the SDK's own CreateRuleSetDetails.UnmarshalJSON (which
	// runs UnmarshalPolymorphicJSON per item) by decoding a combined {name, items} body.
	var details lb.CreateRuleSetDetails
	body, err := json.Marshal(struct {
		Name  string          `json:"name"`
		Items json.RawMessage `json:"items"`
	}{Name: name, Items: json.RawMessage(rawItems)})
	if err != nil {
		return lbn.ErrorResult(fmt.Sprintf(`rules must be a JSON array of rule objects, each with an "action" (e.g. ADD_HTTP_REQUEST_HEADER, REDIRECT, ALLOW): %s`, err.Error())), nil
	}
	if err := json.Unmarshal(body, &details); err != nil {
		return lbn.ErrorResult(fmt.Sprintf(`rules must be a JSON array of rule objects, each with an "action" (e.g. ADD_HTTP_REQUEST_HEADER, REDIRECT, ALLOW): %s`, err.Error())), nil
	}
	if len(details.Items) == 0 {
		return lbn.ErrorResult("at least one rule item is required"), nil
	}
	resp, err := client.CreateRuleSet(lbn.Context(), lb.CreateRuleSetRequest{
		LoadBalancerId:       &lbID,
		CreateRuleSetDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Creating rule set %q on load balancer %s — poll work request %s", name, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"rule_set_name":   name,
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
