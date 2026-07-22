// Package oracle_notifications_subscription_confirm confirms a pending subscription using the
// confirmation token OCI sends the subscriber (in the confirmation email/message). Email and
// HTTPS subscriptions stay PENDING until confirmed — this completes that handshake so delivery
// can begin.
package oracle_notifications_subscription_confirm

import (
	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Confirm Subscription"
	Description  = "Confirm a pending Notifications subscription using the token from the confirmation link OCI sends the subscriber. Email and HTTPS subscriptions stay PENDING until confirmed — this completes the handshake so delivery can start."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "subscription_ocid", Type: core.ConnectionTypeString, Label: "Subscription OCID", Placeholder: "ocid1.onssubscription.oc1..aaaa… to confirm", Required: true},
	{Name: "token", Type: core.ConnectionTypeString, Label: "Confirmation Token", Placeholder: "The token from the confirmation link OCI sent the subscriber", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "The subscription's protocol", Required: true, Options: []core.ConnectionOption{
		{Name: "Email", Value: "EMAIL"},
		{Name: "SMS", Value: "SMS"},
		{Name: "HTTPS (custom URL)", Value: "CUSTOM_HTTPS"},
		{Name: "Slack", Value: "SLACK"},
		{Name: "PagerDuty", Value: "PAGERDUTY"},
		{Name: "Oracle Function", Value: "ORACLE_FUNCTIONS"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription OCID"},
	{Name: "topic_id", Type: core.ConnectionTypeString, Label: "Topic OCID"},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Endpoint"},
	{Name: "unsubscribe_url", Type: core.ConnectionTypeString, Label: "Unsubscribe URL"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Confirmation Message"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ons.DataPlaneClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := ons.RequiredString("subscription_ocid", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	token, err := ons.RequiredString("token", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	protocol, err := ons.RequiredString("protocol", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetConfirmSubscription(ons.Context(), onssdk.GetConfirmSubscriptionRequest{
		Id:       &id,
		Token:    &token,
		Protocol: &protocol,
	})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	r := resp.ConfirmationResult
	return ons.Result("Confirmed subscription", map[string]interface{}{
		"subscription_id": ons.Str(r.SubscriptionId),
		"topic_id":        ons.Str(r.TopicId),
		"endpoint":        ons.Str(r.Endpoint),
		"unsubscribe_url": ons.Str(r.UnsubscribeUrl),
		"message":         ons.Str(r.Message),
	}), nil
}
