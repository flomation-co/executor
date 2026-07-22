// Package oracle_notifications_subscription_update changes a subscription's delivery
// backoff-retry policy and/or its freeform tags. A subscription's protocol and endpoint
// are fixed once created — recreate the subscription to change those.
package oracle_notifications_subscription_update

import (
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Update Subscription"
	Description  = "Update a Notifications subscription — its delivery backoff-retry ceiling and/or freeform tags. A subscription's protocol and endpoint are fixed once created; recreate the subscription to change those."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (optional — scopes the picker)"},
	{Name: "subscription_ocid", Type: core.ConnectionTypeString, Label: "Subscription OCID", Placeholder: "ocid1.onssubscription.oc1..aaaa… to update", Required: true},
	{Name: "max_retry_duration_ms", Type: core.ConnectionTypeString, Label: "Max Retry Duration (ms)", Placeholder: "Backoff retry ceiling in whole milliseconds, e.g. 7200000 (2 hours) — optional"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	details := onssdk.UpdateSubscriptionDetails{}
	if raw := strings.TrimSpace(ons.OptionalString("max_retry_duration_ms", inputs)); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return ons.ErrorResult("max retry duration ms must be a whole number of milliseconds"), nil
		}
		details.DeliveryPolicy = &onssdk.DeliveryPolicy{
			BackoffRetryPolicy: &onssdk.BackoffRetryPolicy{
				MaxRetryDuration: &n,
				PolicyType:       onssdk.BackoffRetryPolicyPolicyTypeExponential,
			},
		}
	}
	if tags, err := ons.FreeformTags("tags", inputs); err != nil {
		return ons.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	_, err = client.UpdateSubscription(ons.Context(), onssdk.UpdateSubscriptionRequest{
		SubscriptionId:            &id,
		UpdateSubscriptionDetails: details,
	})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	return ons.Result("Updated subscription "+id, map[string]interface{}{"id": id}), nil
}
