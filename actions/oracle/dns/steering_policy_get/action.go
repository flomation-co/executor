// Package oracle_dns_steering_policy_get reads one steering policy by OCID.
package oracle_dns_steering_policy_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Get Steering Policy"
	Description  = "Fetch a single Oracle Cloud DNS steering policy by OCID — its template, TTL and rules."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the steering-policy picker)"},
	{Name: "steering_policy_ocid", Type: core.ConnectionTypeString, Label: "Steering Policy OCID", Placeholder: "ocid1.dns-steering-policy.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "steering_policy", Type: core.ConnectionTypeObject, Label: "Steering Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Steering Policy OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "steering_policy_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetSteeringPolicy(dnsn.Context(), dns.GetSteeringPolicyRequest{SteeringPolicyId: &id})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	policy := dnsn.SummariseSteeringPolicy(&resp.SteeringPolicy)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Steering policy %q (%s)", policy["display_name"], policy["template"]),
		"steering_policy": policy,
		"id":              policy["id"],
		"success":         true,
	}, nil
}
