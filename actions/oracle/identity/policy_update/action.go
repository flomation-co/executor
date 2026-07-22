// Package oracle_identity_policy_update updates an IAM policy's description, statements or freeform tags.
package oracle_identity_policy_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Update Policy"
	Description  = "Update an Oracle Cloud IAM policy — its description, statements or freeform tags. Statements REPLACE the policy wholesale, so leave them blank to keep the current ones (they are re-sent unchanged for you)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the policy picker)"},
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Policy OCID", Placeholder: "ocid1.policy.oc1..aaaa… of the policy to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "statements", Type: core.ConnectionTypeText, Label: "Statements (one per line)", Placeholder: "Leave blank to keep the current statements. Anything you enter REPLACES them all.\nAllow group Ops to read instances in compartment Prod"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"team":"ops"} — replaces the freeform tags (leave blank to keep them)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy", Type: core.ConnectionTypeObject, Label: "Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Policy OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "policy_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdatePolicyDetails{}

	// Description — overlay only when supplied; a nil pointer leaves it unchanged.
	if desc := strings.TrimSpace(iam.OptionalString("description", inputs)); desc != "" {
		details.Description = &desc
	}

	// Freeform tags — overlay only when supplied; nil leaves them unchanged.
	tags, err := iam.FreeformTags("tags", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	details.FreeformTags = tags

	// Statements REPLACE wholesale: if the operator supplies any, use them; if blank,
	// READ-MODIFY-WRITE — GET the current policy and re-send its statements so an update
	// that only touches the description/tags never wipes the existing rules.
	statements := iam.InputLines("statements", inputs)
	replaced := len(statements) > 0
	if !replaced {
		cur, err := client.GetPolicy(iam.Context(), identity.GetPolicyRequest{PolicyId: &id})
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		statements = cur.Policy.Statements
	}
	details.Statements = statements

	resp, err := client.UpdatePolicy(iam.Context(), identity.UpdatePolicyRequest{PolicyId: &id, UpdatePolicyDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	policy := iam.SummarisePolicy(&resp.Policy)
	verb := "kept"
	if replaced {
		verb = "replaced"
	}
	return iam.Result(
		fmt.Sprintf("Updated policy %q (%s %d statement(s))", policy["name"], verb, len(statements)),
		map[string]interface{}{"policy": policy, "id": policy["id"]},
	), nil
}
