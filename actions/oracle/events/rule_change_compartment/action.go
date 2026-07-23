// Package oracle_events_rule_change_compartment moves an Events rule from one compartment to
// another. The rule keeps its OCID; only its compartment placement (for access control and
// billing) changes.
package oracle_events_rule_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	ev "flomation.app/automate/executor/actions/oracle/events"

	"github.com/oracle/oci-go-sdk/v65/events"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Events: Change Rule Compartment"
	Description  = "Move an Events rule into a different compartment — the rule keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bolt"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "rule_ocid", Type: core.ConnectionTypeString, Label: "Rule OCID", Placeholder: "ocid1.eventrule.oc1..aaaa… (the rule to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the rule)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Rule OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ev.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	ruleID, err := ev.RequiredString("rule_ocid", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}
	destination, err := ev.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeRuleCompartment(ev.Context(), events.ChangeRuleCompartmentRequest{
		RuleId: &ruleID,
		ChangeRuleCompartmentDetails: events.ChangeRuleCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return ev.ErrorResult(auth.OCIError(err)), nil
	}

	return ev.Result(fmt.Sprintf("Moved rule %s into compartment %s", ruleID, destination), map[string]interface{}{
		"id":                         ruleID,
		"destination_compartment_id": destination,
	}), nil
}
