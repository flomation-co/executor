// Package oracle_notifications_subscription_resend_confirmation resends the confirmation
// details for a Notifications subscription. Email and HTTPS subscriptions stay PENDING until
// the subscriber confirms — this re-sends that confirmation message.
package oracle_notifications_subscription_resend_confirmation

import (
	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Resend Subscription Confirmation"
	Description  = "Resend the confirmation message for a Notifications subscription. Email and HTTPS subscriptions stay PENDING until the subscriber confirms — use this to re-send that confirmation."
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
	{Name: "subscription_ocid", Type: core.ConnectionTypeString, Label: "Subscription OCID", Placeholder: "ocid1.onssubscription.oc1..aaaa… to resend confirmation for", Required: true},
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
	id, err := ons.RequiredString("subscription_ocid", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	resp, err := client.ResendSubscriptionConfirmation(ons.Context(), onssdk.ResendSubscriptionConfirmationRequest{Id: &id})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	sub := ons.SummariseSubscription(&resp.Subscription)
	return ons.Result("Resent confirmation for the subscription", map[string]interface{}{
		"subscription": sub, "id": sub["id"], "lifecycle_state": sub["lifecycle_state"],
	}), nil
}
