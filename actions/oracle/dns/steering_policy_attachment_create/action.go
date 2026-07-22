// Package oracle_dns_steering_policy_attachment_create attaches a steering policy to a
// domain within a public zone, so DNS queries for that domain are answered by the
// policy's traffic-management logic. Steering-policy attachments are GLOBAL-only.
package oracle_dns_steering_policy_attachment_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Create Steering Policy Attachment"
	Description  = "Attach a steering policy to a domain within a public zone, so DNS queries for that domain are answered by the policy's traffic-management logic. Returns a work-request id when provisioning is asynchronous."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "steering_policy_ocid", Type: core.ConnectionTypeString, Label: "Steering Policy OCID", Placeholder: "ocid1.dns-steering-policy.oc1..aaaa… — the policy to attach", Required: true},
	{Name: "zone_ocid", Type: core.ConnectionTypeString, Label: "Zone OCID", Placeholder: "ocid1.dns-zone.oc1..aaaa… — must be a public zone", Required: true},
	{Name: "domain_name", Type: core.ConnectionTypeString, Label: "Domain Name", Placeholder: "The domain within the zone, e.g. www.example.com", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the attachment (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment", Type: core.ConnectionTypeObject, Label: "Steering Policy Attachment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	steeringPolicyID, err := dnsn.RequiredString("steering_policy_ocid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	zoneID, err := dnsn.RequiredString("zone_ocid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	domainName, err := dnsn.RequiredString("domain_name", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	details := dns.CreateSteeringPolicyAttachmentDetails{
		SteeringPolicyId: &steeringPolicyID,
		ZoneId:           &zoneID,
		DomainName:       &domainName,
	}
	if v := dnsn.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	resp, err := client.CreateSteeringPolicyAttachment(dnsn.Context(), dns.CreateSteeringPolicyAttachmentRequest{CreateSteeringPolicyAttachmentDetails: details})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	attachment := dnsn.SummariseSteeringPolicyAttachment(&resp.SteeringPolicyAttachment)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Attached steering policy to %q (%s)", domainName, attachment["lifecycle_state"]),
		"attachment":      attachment,
		"id":              attachment["id"],
		"work_request_id": dnsn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
