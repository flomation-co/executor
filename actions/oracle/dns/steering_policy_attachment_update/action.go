// Package oracle_dns_steering_policy_attachment_update renames a DNS steering policy attachment.
package oracle_dns_steering_policy_attachment_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Update Steering Policy Attachment"
	Description  = "Update an Oracle Cloud DNS steering policy attachment's friendly display name, leaving its policy, zone and domain binding untouched."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+link"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the attachment picker)"},
	{Name: "attachment_ocid", Type: core.ConnectionTypeString, Label: "Steering Policy Attachment OCID", Placeholder: "ocid1.dnssteeringpolicyattachment.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name (leave blank to keep the current name)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment", Type: core.ConnectionTypeObject, Label: "Steering Policy Attachment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "attachment_ocid")
	if errResult != nil {
		return errResult, nil
	}

	// Read-modify-write: UpdateSteeringPolicyAttachmentDetails is a full-replace PUT.
	// Its only field is DisplayName, so read the current value and re-send it when the
	// operator leaves display_name blank, overlaying only when they supply a new name.
	current, err := client.GetSteeringPolicyAttachment(dnsn.Context(), dns.GetSteeringPolicyAttachmentRequest{SteeringPolicyAttachmentId: &id})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}

	details := dns.UpdateSteeringPolicyAttachmentDetails{DisplayName: current.SteeringPolicyAttachment.DisplayName}
	if name := dnsn.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.UpdateSteeringPolicyAttachment(dnsn.Context(), dns.UpdateSteeringPolicyAttachmentRequest{
		SteeringPolicyAttachmentId:            &id,
		UpdateSteeringPolicyAttachmentDetails: details,
	})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}

	attachment := dnsn.SummariseSteeringPolicyAttachment(&resp.SteeringPolicyAttachment)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Steering policy attachment %q is %s", attachment["display_name"], attachment["lifecycle_state"]),
		"attachment":      attachment,
		"id":              attachment["id"],
		"lifecycle_state": attachment["lifecycle_state"],
		"success":         true,
	}, nil
}
