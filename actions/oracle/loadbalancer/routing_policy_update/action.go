// Package oracle_loadbalancer_routing_policy_update replaces a routing policy's ordered
// rules on a load balancer. The supplied array overwrites the policy's entire list of
// routing rules. Asynchronous — returns a work-request id.
package oracle_loadbalancer_routing_policy_update

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
	Name         = "OCI Load Balancer: Update Routing Policy"
	Description  = "Update a routing policy on an Oracle Cloud load balancer — replaces the policy's whole ordered list of routing rules with the supplied array. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "routing_policy_name", Type: core.ConnectionTypeString, Label: "Routing Policy Name", Placeholder: "The name of the routing policy to update, e.g. example_routing_policy", Required: true},
	{Name: "condition_language_version", Type: core.ConnectionTypeString, Label: "Condition Language Version", Placeholder: "V1 (the only supported version)"},
	{Name: "rules_json", Type: core.ConnectionTypeString, Label: "Rules (JSON)", Placeholder: `[{"name":"route_to_admin","condition":"any(http.request.url.path sw '/admin')","actions":[{"name":"FORWARD_TO_BACKENDSET","backendSetName":"admin-servers"}]}]`, Required: true},
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
	raw, err := lbn.RequiredString("rules_json", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// RoutingRule carries polymorphic Actions; its SDK UnmarshalJSON resolves each action
	// concretely, so a plain json.Unmarshal into the slice is correct.
	var rules []lb.RoutingRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return lbn.ErrorResult(fmt.Sprintf("rules must be a JSON array of routing rules, e.g. [{\"name\":\"route_to_admin\",\"condition\":\"any(http.request.url.path sw '/admin')\",\"actions\":[{\"name\":\"FORWARD_TO_BACKENDSET\",\"backendSetName\":\"admin-servers\"}]}]: %s", err.Error())), nil
	}
	if len(rules) == 0 {
		return lbn.ErrorResult("rules must contain at least one routing rule"), nil
	}
	details := lb.UpdateRoutingPolicyDetails{Rules: rules}
	// Condition language version defaults to V1 (the only supported version) when blank.
	clv := lbn.OptionalString("condition_language_version", inputs)
	if clv == "" {
		clv = "V1"
	}
	details.ConditionLanguageVersion = lb.UpdateRoutingPolicyDetailsConditionLanguageVersionEnum(clv)
	// Replace-semantics: UpdateRoutingPolicyDetails overwrites the policy's entire ordered
	// list of routing rules, so the supplied array becomes the policy's complete rule set.
	resp, err := client.UpdateRoutingPolicy(lbn.Context(), lb.UpdateRoutingPolicyRequest{
		LoadBalancerId:             &lbID,
		RoutingPolicyName:          &name,
		UpdateRoutingPolicyDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Updating routing policy %q (%d rule(s)) on load balancer %s — poll work request %s", name, len(rules), lbID, lbn.Str(resp.OpcWorkRequestId)),
		"routing_policy_name": name,
		"work_request_id":     lbn.Str(resp.OpcWorkRequestId),
		"success":             true,
	}, nil
}
