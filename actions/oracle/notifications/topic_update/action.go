// Package oracle_notifications_topic_update updates a Notifications topic's description and tags.
package oracle_notifications_topic_update

import (
	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Update Topic"
	Description  = "Update the description and free-form tags of an Oracle Cloud Notifications topic."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID (scopes the picker)", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)"},
	{Name: "topic_ocid", Type: core.ConnectionTypeString, Label: "Topic OCID", Placeholder: "ocid1.onstopic.oc1..aaaa…", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description for the topic (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Free-form Tags", Placeholder: "JSON object of string values, e.g. {\"env\":\"prod\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "topic", Type: core.ConnectionTypeObject, Label: "Topic"},
	{Name: "topic_id", Type: core.ConnectionTypeString, Label: "Topic OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle state"},
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
	// Description is mandatory:"true" in the ONS UpdateTopic model, so a nil serialises as
	// "description":null — which clears the topic's description. On a blank (e.g. tags-only)
	// update, read the current value and resend it so nothing is wiped.
	description := ons.OptionalString("description", inputs)
	if description == "" {
		cur, err := client.GetTopic(ons.Context(), onssdk.GetTopicRequest{TopicId: &id})
		if err != nil {
			return ons.ErrorResult(auth.OCIError(err)), nil
		}
		description = ons.Str(cur.Description)
	}
	details := onssdk.TopicAttributesDetails{Description: &description}
	tags, err := ons.FreeformTags("tags", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	details.FreeformTags = tags
	resp, err := client.UpdateTopic(ons.Context(), onssdk.UpdateTopicRequest{
		TopicId:                &id,
		TopicAttributesDetails: details,
	})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	topic := ons.SummariseTopic(&resp.NotificationTopic)
	return ons.Result("Topic updated", map[string]interface{}{
		"topic":           topic,
		"topic_id":        topic["topic_id"],
		"lifecycle_state": topic["lifecycle_state"],
	}), nil
}
