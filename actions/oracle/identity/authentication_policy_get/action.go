// Package oracle_identity_authentication_policy_get reads the compartment's authentication (password/network) policy.
package oracle_identity_authentication_policy_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Authentication Policy"
	Description  = "Fetch the Oracle Cloud IAM authentication policy for a compartment (defaulting to the tenancy) — its password complexity rules and the network sources allowed to sign in."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (the authentication policy is read from this compartment)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "authentication_policy", Type: core.ConnectionTypeObject, Label: "Authentication Policy"},
	{Name: "compartment_id", Type: core.ConnectionTypeString, Label: "Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartmentID := auth.CompartmentOrTenancy()
	resp, err := client.GetAuthenticationPolicy(iam.Context(), identity.GetAuthenticationPolicyRequest{CompartmentId: &compartmentID})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	policy := map[string]interface{}{
		"compartment_id": iam.Str(resp.CompartmentId),
	}
	minLen := "unset"
	if pp := resp.PasswordPolicy; pp != nil {
		policy["password_policy"] = map[string]interface{}{
			"minimum_password_length":          iam.IntOrNil(pp.MinimumPasswordLength),
			"is_uppercase_characters_required": pp.IsUppercaseCharactersRequired != nil && *pp.IsUppercaseCharactersRequired,
			"is_lowercase_characters_required": pp.IsLowercaseCharactersRequired != nil && *pp.IsLowercaseCharactersRequired,
			"is_numeric_characters_required":   pp.IsNumericCharactersRequired != nil && *pp.IsNumericCharactersRequired,
			"is_special_characters_required":   pp.IsSpecialCharactersRequired != nil && *pp.IsSpecialCharactersRequired,
			"is_username_containment_allowed":  pp.IsUsernameContainmentAllowed != nil && *pp.IsUsernameContainmentAllowed,
		}
		if pp.MinimumPasswordLength != nil {
			minLen = fmt.Sprintf("%d", *pp.MinimumPasswordLength)
		}
	}
	if np := resp.NetworkPolicy; np != nil {
		policy["network_policy"] = map[string]interface{}{
			"network_source_ids": np.NetworkSourceIds,
		}
	}

	return iam.Result(
		fmt.Sprintf("Authentication policy for compartment %q — minimum password length %s", compartmentID, minLen),
		map[string]interface{}{"authentication_policy": policy, "compartment_id": iam.Str(resp.CompartmentId)},
	), nil
}
