// Package oracle_dns_steering_policy_attachment_list lists the DNS steering-policy
// attachments in a compartment, optionally narrowed by steering policy, zone or domain.
package oracle_dns_steering_policy_attachment_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: List Steering Policy Attachments"
	Description  = "List the Oracle Cloud DNS steering-policy attachments in a compartment, optionally filtered by steering policy, zone or domain. Walks pagination up to a safe cap."
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
	{Name: "steering_policy_ocid", Type: core.ConnectionTypeString, Label: "Steering Policy OCID", Placeholder: "Only attachments of this steering policy (optional)"},
	{Name: "zone_ocid", Type: core.ConnectionTypeString, Label: "Zone OCID", Placeholder: "Only attachments on this zone (optional)"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "Only the attachment on this exact domain, e.g. www.example.com (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachments", Type: core.ConnectionTypeObject, Label: "Attachments"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.ListSteeringPolicyAttachmentsRequest{CompartmentId: &compartment}
	if v := strings.TrimSpace(dnsn.OptionalString("steering_policy_ocid", inputs)); v != "" {
		req.SteeringPolicyId = &v
	}
	if v := strings.TrimSpace(dnsn.OptionalString("zone_ocid", inputs)); v != "" {
		req.ZoneId = &v
	}
	if v := strings.TrimSpace(dnsn.OptionalString("domain", inputs)); v != "" {
		req.Domain = &v
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= dnsn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListSteeringPolicyAttachments(dnsn.Context(), req)
		if err != nil {
			return dnsn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, dnsn.SummariseSteeringPolicyAttachmentSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d steering-policy attachment(s)", len(out)),
		"attachments": out,
		"count":       fmt.Sprintf("%d", len(out)),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
