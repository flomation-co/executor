// Package oracle_notifications_topic_get retrieves a single Notifications topic by OCID.
package oracle_notifications_topic_get

import (
	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Get Topic"
	Description  = "Fetch a single Oracle Cloud Notifications topic by its OCID."
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
	{Name: "topic_ocid", Type: core.ConnectionTypeString, Label: "Topic OCID", Placeholder: "ocid1.onstopic.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "topic", Type: core.ConnectionTypeObject, Label: "Topic"},
	{Name: "topic_id", Type: core.ConnectionTypeString, Label: "Topic OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ons.ControlPlaneClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := ons.RequiredString("topic_ocid", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetTopic(ons.Context(), onssdk.GetTopicRequest{TopicId: &id})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	topic := ons.SummariseTopic(&resp.NotificationTopic)
	return ons.Result("Retrieved topic "+ons.Str(resp.NotificationTopic.Name), map[string]interface{}{
		"topic":           topic,
		"topic_id":        ons.Str(resp.NotificationTopic.TopicId),
		"lifecycle_state": string(resp.NotificationTopic.LifecycleState),
	}), nil
}
