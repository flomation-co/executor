// Package oracle_events_rule_delete deletes an Events rule by its OCID, stopping it from matching
// and fanning out any further events.
package oracle_events_rule_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	ev "flomation.app/automate/executor/actions/oracle/events"

	"github.com/oracle/oci-go-sdk/v65/events"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Events: Delete Rule"
	Description  = "Delete an Events rule by its OCID — it stops matching events and fanning them out."
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
	{Name: "rule_ocid", Type: core.ConnectionTypeString, Label: "Rule OCID", Placeholder: "ocid1.eventrule.oc1..aaaa… of the rule to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Rule OCID"},
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

	_, err = client.DeleteRule(ev.Context(), events.DeleteRuleRequest{RuleId: &ruleID})
	if err != nil {
		return ev.ErrorResult(auth.OCIError(err)), nil
	}
	return ev.Result(fmt.Sprintf("Deleted rule %s", ruleID), map[string]interface{}{
		"id": ruleID,
	}), nil
}
