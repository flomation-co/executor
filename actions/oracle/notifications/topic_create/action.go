// Package oracle_notifications_topic_create creates a Notifications topic — the fan-out point
// that subscriptions attach to and messages are published to.
package oracle_notifications_topic_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	ons "flomation.app/automate/executor/actions/oracle/notifications"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Notifications: Create Topic"
	Description  = "Create an Oracle Cloud Notifications topic — the fan-out point that subscriptions attach to and messages are published to. Poll Get Topic until ACTIVE."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Topic Name", Placeholder: "A unique name within the compartment (letters, numbers, hyphens, underscores)", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this topic is for (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	name, err := ons.RequiredString("name", inputs)
	if err != nil {
		return ons.ErrorResult(err.Error()), nil
	}
	details := onssdk.CreateTopicDetails{Name: &name, CompartmentId: &compartment}
	if d := ons.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}
	if tags, err := ons.FreeformTags("tags", inputs); err != nil {
		return ons.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateTopic(ons.Context(), onssdk.CreateTopicRequest{CreateTopicDetails: details})
	if err != nil {
		return ons.ErrorResult(auth.OCIError(err)), nil
	}
	topic := ons.SummariseTopic(&resp.NotificationTopic)
	return ons.Result(fmt.Sprintf("Creating topic %q (%s) — poll Get Topic until ACTIVE", name, topic["lifecycle_state"]), map[string]interface{}{
		"topic": topic, "topic_id": topic["topic_id"], "lifecycle_state": topic["lifecycle_state"],
	}), nil
}
