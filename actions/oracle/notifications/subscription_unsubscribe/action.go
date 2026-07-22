// Package oracle_notifications_subscription_unsubscribe unsubscribes an endpoint from a
// Notifications topic using the token and protocol from the unsubscribe link OCI issues in
// each notification. Like confirming a subscription, this is done via the data-plane
// GetUnsubscription call — the token and protocol must match the subscription being removed.
package oracle_notifications_subscription_unsubscribe

import (
	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Unsubscribe"
	Description  = "Unsubscribe an endpoint from a Notifications topic using the subscription's OCID plus the token and protocol from the unsubscribe link OCI includes in every notification."
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
	{Name: "subscription_ocid", Type: core.ConnectionTypeString, Label: "Subscription OCID", Placeholder: "ocid1.onssubscription.oc1..aaaa… to unsubscribe", Required: true},
	{Name: "token", Type: core.ConnectionTypeString, Label: "Token", Placeholder: "The token from the unsubscribe link OCI included in the notification", Required: true},
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
	{Name: "result", Type: core.ConnectionTypeString, Label: "Unsubscribe result"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subscription OCID"},
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
	resp, err := client.GetUnsubscription(ons.Context(), onssdk.GetUnsubscriptionRequest{Id: &id, Token: &token, Protocol: &protocol})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	return ons.Result("Unsubscribed", map[string]interface{}{
		"result": ons.Str(resp.Value),
		"id":     id,
	}), nil
}
