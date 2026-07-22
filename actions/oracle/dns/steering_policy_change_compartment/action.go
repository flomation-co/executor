// Package oracle_dns_steering_policy_change_compartment moves a DNS steering policy into a different compartment.
package oracle_dns_steering_policy_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Change Steering Policy Compartment"
	Description  = "Move an Oracle Cloud DNS steering policy into a different compartment by its OCID."
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
	{Name: "target_compartment_ocid", Type: core.ConnectionTypeString, Label: "Target Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — the compartment to move the policy into", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Steering Policy OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "steering_policy_ocid")
	if errResult != nil {
		return errResult, nil
	}
	target, err := dnsn.RequiredString("target_compartment_ocid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.ChangeSteeringPolicyCompartmentRequest{SteeringPolicyId: &id}
	req.CompartmentId = &target
	resp, err := client.ChangeSteeringPolicyCompartment(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	// Async (202) — surface the work-request id like the zone/view siblings.
	result := dnsn.AsyncResult(fmt.Sprintf("Move requested for steering policy %s to compartment %s", id, target), dnsn.Str(resp.OpcWorkRequestId))
	result["id"] = id
	return result, nil
}
