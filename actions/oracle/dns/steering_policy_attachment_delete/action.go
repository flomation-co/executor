// Package oracle_dns_steering_policy_attachment_delete detaches a steering policy from
// a domain by deleting the attachment. Synchronous (204) — no work request.
package oracle_dns_steering_policy_attachment_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Delete Steering Policy Attachment"
	Description  = "Detach a steering policy from a domain by deleting its attachment (by OCID). Synchronous — the attachment is gone once this succeeds."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+link"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the attachment picker)"},
	{Name: "attachment_ocid", Type: core.ConnectionTypeString, Label: "Steering Policy Attachment OCID", Placeholder: "ocid1.dnssteeringpolicyattachment.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "attachment_ocid")
	if errResult != nil {
		return errResult, nil
	}
	req := dns.DeleteSteeringPolicyAttachmentRequest{SteeringPolicyAttachmentId: &id}
	if _, err := client.DeleteSteeringPolicyAttachment(dnsn.Context(), req); err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted steering policy attachment %s", id),
		"id":          id,
		"success":     true,
	}, nil
}
