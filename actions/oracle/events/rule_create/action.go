// Package oracle_events_rule_create creates an Events rule: a condition that matches OCI event
// types (a resource created, updated or deleted, …) and one action that fans the matched event
// out to a Notifications topic, a stream, or a function.
package oracle_events_rule_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	ev "flomation.app/automate/executor/actions/oracle/events"

	"github.com/oracle/oci-go-sdk/v65/events"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Events: Create Rule"
	Description  = "Create an Events rule — a condition (a JSON event-matching filter) plus one action that routes matched events to a Notifications topic (ONS), a stream (OSS) or a function (FAAS)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bolt"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the rule", Required: true},
	{Name: "condition", Type: core.ConnectionTypeText, Label: "Condition (JSON)", Placeholder: "{\"eventType\":\"com.oraclecloud.objectstorage.createobject\"}", Required: true},
	{Name: "action_type", Type: core.ConnectionTypeString, Label: "Action Type", Placeholder: "Where to send matched events", Required: true, Options: []core.ConnectionOption{
		{Name: "Notifications topic (ONS)", Value: "ONS"},
		{Name: "Stream (OSS)", Value: "OSS"},
		{Name: "Function (FAAS)", Value: "FAAS"},
	}},
	{Name: "action_target_id", Type: core.ConnectionTypeString, Label: "Action Target OCID", Placeholder: "Topic / stream / function OCID for the action", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Enable the rule now (default true)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule", Type: core.ConnectionTypeObject, Label: "Rule"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Rule OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ev.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}
	name, err := ev.RequiredString("display_name", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}
	condition, err := ev.RequiredString("condition", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}
	actionType, err := ev.RequiredString("action_type", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}
	target, err := ev.RequiredString("action_target_id", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}

	enabled := true
	if p := ev.OptionalBoolPtr("is_enabled", inputs); p != nil {
		enabled = *p
	}
	var descPtr *string
	if d := ev.OptionalString("description", inputs); d != "" {
		descPtr = &d
	}

	// Build the single action. The concrete Create*ActionDetails type carries its own actionType
	// discriminator via MarshalJSON, so we only set the target id + enabled flag.
	var action events.ActionDetails
	switch actionType {
	case "ONS":
		action = events.CreateNotificationServiceActionDetails{IsEnabled: &enabled, Description: descPtr, TopicId: &target}
	case "OSS":
		action = events.CreateStreamingServiceActionDetails{IsEnabled: &enabled, Description: descPtr, StreamId: &target}
	case "FAAS":
		action = events.CreateFaaSActionDetails{IsEnabled: &enabled, Description: descPtr, FunctionId: &target}
	default:
		return ev.ErrorResult("action type must be ONS, OSS or FAAS"), nil
	}

	details := events.CreateRuleDetails{
		DisplayName:   &name,
		IsEnabled:     &enabled,
		Condition:     &condition,
		CompartmentId: &compartment,
		Actions:       &events.ActionDetailsList{Actions: []events.ActionDetails{action}},
	}
	if descPtr != nil {
		details.Description = descPtr
	}
	if tags, err := ev.FreeformTags("freeform_tags", inputs); err != nil {
		return ev.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.CreateRule(ev.Context(), events.CreateRuleRequest{CreateRuleDetails: details})
	if err != nil {
		return ev.ErrorResult(auth.OCIError(err)), nil
	}
	rule := ev.SummariseRule(&resp.Rule)
	return ev.Result(fmt.Sprintf("Created rule %q (%s)", rule["display_name"], rule["lifecycle_state"]), map[string]interface{}{
		"rule": rule, "id": rule["id"], "lifecycle_state": rule["lifecycle_state"],
	}), nil
}
