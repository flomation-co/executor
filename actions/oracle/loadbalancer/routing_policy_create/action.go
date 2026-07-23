// Package oracle_loadbalancer_routing_policy_create adds a routing policy — an
// ordered list of condition→backend-set routing rules — to a load balancer.
// Asynchronous — returns a work-request id.
package oracle_loadbalancer_routing_policy_create

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Create Routing Policy"
	Description  = "Add a routing policy — an ordered list of condition→backend-set routing rules — to an Oracle Cloud load balancer. Routing rules apply only to HTTP/HTTPS traffic. Asynchronous."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "routing_policy_name", Type: core.ConnectionTypeString, Label: "Routing Policy Name", Placeholder: "A name unique within the load balancer, e.g. api_routing", Required: true},
	{Name: "condition_language_version", Type: core.ConnectionTypeString, Label: "Condition Language Version", Placeholder: "Version the rule conditions are composed in (default V1)"},
	{Name: "rules_json", Type: core.ConnectionTypeString, Label: "Rules (JSON)", Placeholder: `Ordered rule list, e.g. [{"name":"r1","condition":"any(http.request.url.path eq '/api')","actions":[{"name":"FORWARD_TO_BACKENDSET","backendSetName":"api-servers"}]}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "routing_policy_name", Type: core.ConnectionTypeString, Label: "Routing Policy Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := lbn.RequiredString("routing_policy_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// Condition language version defaults to V1 (the only value the API currently accepts).
	version := lb.CreateRoutingPolicyDetailsConditionLanguageVersionV1
	if v := strings.TrimSpace(lbn.OptionalString("condition_language_version", inputs)); v != "" {
		version = lb.CreateRoutingPolicyDetailsConditionLanguageVersionEnum(v)
	}
	// Polymorphic body: RoutingRule.UnmarshalJSON resolves each action ({name, backendSetName}).
	rawRules, err := lbn.RequiredString("rules_json", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	var rules []lb.RoutingRule
	if err := json.Unmarshal([]byte(rawRules), &rules); err != nil {
		return lbn.ErrorResult(fmt.Sprintf(`rules JSON must be an array of routing rules, e.g. [{"name":"r1","condition":"any(http.request.url.path eq '/api')","actions":[{"name":"FORWARD_TO_BACKENDSET","backendSetName":"api-servers"}]}]: %s`, err.Error())), nil
	}
	if len(rules) == 0 {
		return lbn.ErrorResult("at least one routing rule is required"), nil
	}
	resp, err := client.CreateRoutingPolicy(lbn.Context(), lb.CreateRoutingPolicyRequest{
		LoadBalancerId: &lbID,
		CreateRoutingPolicyDetails: lb.CreateRoutingPolicyDetails{
			Name:                     &name,
			ConditionLanguageVersion: version,
			Rules:                    rules,
		},
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Creating routing policy %q (%d rule(s)) on load balancer %s — poll work request %s", name, len(rules), lbID, lbn.Str(resp.OpcWorkRequestId)),
		"routing_policy_name": name,
		"work_request_id":     lbn.Str(resp.OpcWorkRequestId),
		"success":             true,
	}, nil
}
