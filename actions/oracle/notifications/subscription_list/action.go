// Package oracle_notifications_subscription_list lists the Notifications subscriptions in a
// compartment, optionally narrowed to a single topic.
package oracle_notifications_subscription_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: List Subscriptions"
	Description  = "List the Oracle Cloud Notifications subscriptions in a compartment, optionally narrowed to a single topic. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bell"
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
	{Name: "topic_ocid", Type: core.ConnectionTypeString, Label: "Topic OCID", Placeholder: "ocid1.onstopic.oc1..aaaa… to narrow to one topic (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subscriptions", Type: core.ConnectionTypeObject, Label: "Subscriptions"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ons.DataPlaneClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	req := onssdk.ListSubscriptionsRequest{CompartmentId: &compartment}
	if topicID := ons.OptionalString("topic_ocid", inputs); topicID != "" {
		req.TopicId = &topicID
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= ons.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListSubscriptions(ons.Context(), req)
		if err != nil {
			return ons.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, ons.SummariseSubscriptionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return ons.Result(fmt.Sprintf("Found %d subscription(s)", len(out)), map[string]interface{}{
		"subscriptions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
