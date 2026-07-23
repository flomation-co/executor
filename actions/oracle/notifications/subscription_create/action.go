// Package oracle_notifications_subscription_create subscribes an endpoint (email, SMS, an
// HTTPS URL, Slack, PagerDuty, …) to a topic. For email/HTTPS the subscriber must confirm
// before delivery starts — the subscription stays PENDING until then.
package oracle_notifications_subscription_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Create Subscription"
	Description  = "Subscribe an endpoint to a Notifications topic — email, SMS, an HTTPS URL, Slack, PagerDuty or an Oracle Function. For email and HTTPS the subscriber must confirm the subscription before delivery starts (it stays PENDING until then)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bell"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "topic_ocid", Type: core.ConnectionTypeString, Label: "Topic OCID", Placeholder: "ocid1.onstopic.oc1..aaaa… to subscribe to", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "How the subscriber is notified", Required: true, Options: []core.ConnectionOption{
		{Name: "Email", Value: "EMAIL"},
		{Name: "SMS", Value: "SMS"},
		{Name: "HTTPS (custom URL)", Value: "CUSTOM_HTTPS"},
		{Name: "Slack", Value: "SLACK"},
		{Name: "PagerDuty", Value: "PAGERDUTY"},
		{Name: "Oracle Function", Value: "ORACLE_FUNCTIONS"},
	}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Endpoint", Placeholder: "The address for the protocol — email address, phone number, HTTPS URL, Slack webhook, function OCID…", Required: true},
	{Name: "metadata", Type: core.ConnectionTypeString, Label: "Metadata (JSON)", Placeholder: "Protocol-specific metadata, e.g. custom headers for HTTPS (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subscription", Type: core.ConnectionTypeObject, Label: "Subscription"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subscription OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	topicID, err := ons.RequiredString("topic_ocid", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	protocol := strings.ToUpper(strings.TrimSpace(ons.OptionalString("protocol", inputs)))
	if protocol == "" {
		return ons.ErrorResult("protocol is required"), nil
	}
	endpoint, err := ons.RequiredString("endpoint", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	details := onssdk.CreateSubscriptionDetails{
		TopicId:       &topicID,
		CompartmentId: &compartment,
		Protocol:      &protocol,
		Endpoint:      &endpoint,
	}
	if m := ons.OptionalString("metadata", inputs); m != "" {
		details.Metadata = &m
	}
	if tags, err := ons.FreeformTags("tags", inputs); err != nil {
		return ons.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateSubscription(ons.Context(), onssdk.CreateSubscriptionRequest{CreateSubscriptionDetails: details})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	sub := ons.SummariseSubscription(&resp.Subscription)
	return ons.Result(fmt.Sprintf("Subscribed %s endpoint to the topic (%s) — email/HTTPS require the subscriber to confirm", protocol, sub["lifecycle_state"]), map[string]interface{}{
		"subscription": sub, "id": sub["id"], "lifecycle_state": sub["lifecycle_state"],
	}), nil
}
