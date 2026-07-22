// Package oracle_notifications_subscription_get fetches a single Notifications
// subscription by its OCID, returning its protocol, endpoint and lifecycle state
// (PENDING until an email/HTTPS subscriber confirms, ACTIVE once delivering).
package oracle_notifications_subscription_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Get Subscription"
	Description  = "Fetch a single Notifications subscription by its OCID — its topic, protocol, endpoint and lifecycle state (PENDING until an email/HTTPS subscriber confirms, ACTIVE once delivering)."
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
	{Name: "subscription_ocid", Type: core.ConnectionTypeString, Label: "Subscription OCID", Placeholder: "ocid1.onssubscription.oc1..aaaa… to fetch", Required: true},
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
	resp, err := client.GetSubscription(ons.Context(), onssdk.GetSubscriptionRequest{SubscriptionId: &id})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	sub := ons.SummariseSubscription(&resp.Subscription)
	return ons.Result(fmt.Sprintf("Fetched subscription %s (%s) — %s", sub["id"], sub["protocol"], sub["lifecycle_state"]), map[string]interface{}{
		"subscription": sub, "id": sub["id"], "lifecycle_state": sub["lifecycle_state"],
	}), nil
}
