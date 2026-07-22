// Package oracle_events_rule_update applies a partial update to an Events rule: only the display
// name, description, condition and enabled flag you supply are changed; blank fields are left as-is,
// and the rule's existing actions are preserved.
package oracle_events_rule_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	ev "flomation.app/automate/executor/actions/oracle/events"

	"github.com/oracle/oci-go-sdk/v65/events"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Events: Update Rule"
	Description  = "Partially update an Events rule — change only the display name, description, condition (JSON filter) or enabled flag you supply; blank fields are left unchanged and the rule's existing actions are preserved."
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
	{Name: "rule_ocid", Type: core.ConnectionTypeString, Label: "Rule OCID", Placeholder: "ocid1.eventrule.oc1..aaaa… — the rule to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
	{Name: "condition", Type: core.ConnectionTypeText, Label: "Condition (JSON)", Placeholder: "{\"eventType\":\"com.oraclecloud.objectstorage.createobject\"} (leave blank to keep unchanged)"},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Enable/disable the rule (leave blank to keep unchanged)"},
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
	ruleID, err := ev.RequiredString("rule_ocid", inputs)
	if err != nil {
		return ev.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied. Actions is left nil so
	// the rule keeps its existing action list; is_enabled is a *bool (nil = unchanged).
	details := events.UpdateRuleDetails{IsEnabled: ev.OptionalBoolPtr("is_enabled", inputs)}
	if v := ev.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := ev.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}
	if v := ev.OptionalString("condition", inputs); v != "" {
		details.Condition = &v
	}

	resp, err := client.UpdateRule(ev.Context(), events.UpdateRuleRequest{RuleId: &ruleID, UpdateRuleDetails: details})
	if err != nil {
		return ev.ErrorResult(auth.OCIError(err)), nil
	}
	rule := ev.SummariseRule(&resp.Rule)
	return ev.Result(fmt.Sprintf("Updated rule %q (%s)", rule["display_name"], rule["lifecycle_state"]), map[string]interface{}{
		"rule": rule, "id": rule["id"], "lifecycle_state": rule["lifecycle_state"],
	}), nil
}
