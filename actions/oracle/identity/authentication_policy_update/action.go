// Package oracle_identity_authentication_policy_update updates a compartment's IAM password policy.
package oracle_identity_authentication_policy_update

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
	Name         = "OCI Identity: Update Authentication Policy"
	Description  = "Update the Oracle Cloud IAM authentication (password) policy for a compartment — minimum length and the character-class requirements. Reads the current policy first and overlays only the fields you set, leaving the rest (including the network policy) untouched."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (the policy applies at the tenancy root)"},
	{Name: "min_password_length", Type: core.ConnectionTypeInteger, Label: "Minimum Password Length", Placeholder: "e.g. 12 (leave unset to keep unchanged)"},
	{Name: "is_uppercase_required", Type: core.ConnectionTypeBoolean, Label: "Require Uppercase Character", Placeholder: "At least one A–Z required (leave unset to keep unchanged)"},
	{Name: "is_lowercase_required", Type: core.ConnectionTypeBoolean, Label: "Require Lowercase Character", Placeholder: "At least one a–z required (leave unset to keep unchanged)"},
	{Name: "is_numeric_required", Type: core.ConnectionTypeBoolean, Label: "Require Numeric Character", Placeholder: "At least one 0–9 required (leave unset to keep unchanged)"},
	{Name: "is_special_char_required", Type: core.ConnectionTypeBoolean, Label: "Require Special Character", Placeholder: "At least one special character required (leave unset to keep unchanged)"},
	{Name: "is_username_containment_allowed", Type: core.ConnectionTypeBoolean, Label: "Allow Username In Password", Placeholder: "Permit the user name to appear in the password (leave unset to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy", Type: core.ConnectionTypeObject, Label: "Authentication Policy"},
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

	// READ: fetch the current policy so unspecified fields (and the network policy) survive.
	cur, err := client.GetAuthenticationPolicy(iam.Context(), identity.GetAuthenticationPolicyRequest{CompartmentId: &compartmentID})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	// MODIFY: start from the current password policy and overlay only supplied inputs.
	pp := identity.PasswordPolicy{}
	if cur.PasswordPolicy != nil {
		pp = *cur.PasswordPolicy
	}
	var changed []string

	if n, ok, err := iam.OptionalInt("min_password_length", inputs); err != nil {
		return iam.ErrorResult(err.Error()), nil
	} else if ok {
		if n < 1 {
			return iam.ErrorResult("Minimum password length must be a positive whole number."), nil
		}
		v := n
		pp.MinimumPasswordLength = &v
		changed = append(changed, fmt.Sprintf("minimum length %d", n))
	}

	setBool := func(name, label string, dst **bool) {
		if !iam.BoolWasSet(name, inputs) {
			return
		}
		v := iam.OptionalBool(name, inputs, false)
		*dst = &v
		state := "off"
		if v {
			state = "on"
		}
		changed = append(changed, fmt.Sprintf("%s %s", label, state))
	}

	setBool("is_uppercase_required", "uppercase", &pp.IsUppercaseCharactersRequired)
	setBool("is_lowercase_required", "lowercase", &pp.IsLowercaseCharactersRequired)
	setBool("is_numeric_required", "numeric", &pp.IsNumericCharactersRequired)
	setBool("is_special_char_required", "special", &pp.IsSpecialCharactersRequired)
	setBool("is_username_containment_allowed", "username-in-password", &pp.IsUsernameContainmentAllowed)

	if len(changed) == 0 {
		return iam.ErrorResult("Set at least one password-policy field (minimum length or a character requirement) to update — none were provided."), nil
	}

	// WRITE: re-send the (unchanged) network policy so an update never wipes it.
	resp, err := client.UpdateAuthenticationPolicy(iam.Context(), identity.UpdateAuthenticationPolicyRequest{
		CompartmentId: &compartmentID,
		UpdateAuthenticationPolicyDetails: identity.UpdateAuthenticationPolicyDetails{
			PasswordPolicy: &pp,
			NetworkPolicy:  cur.NetworkPolicy,
		},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	policy := map[string]interface{}{"compartment_id": iam.Str(resp.CompartmentId)}
	if p := resp.PasswordPolicy; p != nil {
		password := map[string]interface{}{}
		if p.MinimumPasswordLength != nil {
			password["minimum_password_length"] = *p.MinimumPasswordLength
		}
		if p.IsUppercaseCharactersRequired != nil {
			password["is_uppercase_characters_required"] = *p.IsUppercaseCharactersRequired
		}
		if p.IsLowercaseCharactersRequired != nil {
			password["is_lowercase_characters_required"] = *p.IsLowercaseCharactersRequired
		}
		if p.IsNumericCharactersRequired != nil {
			password["is_numeric_characters_required"] = *p.IsNumericCharactersRequired
		}
		if p.IsSpecialCharactersRequired != nil {
			password["is_special_characters_required"] = *p.IsSpecialCharactersRequired
		}
		if p.IsUsernameContainmentAllowed != nil {
			password["is_username_containment_allowed"] = *p.IsUsernameContainmentAllowed
		}
		policy["password_policy"] = password
	}
	if np := resp.NetworkPolicy; np != nil {
		policy["network_policy"] = map[string]interface{}{"network_source_ids": np.NetworkSourceIds}
	}

	msg := fmt.Sprintf("Updated authentication policy for compartment %s: %s", compartmentID, strings.Join(changed, ", "))
	return iam.Result(msg, map[string]interface{}{"policy": policy, "compartment_id": iam.Str(resp.CompartmentId)}), nil
}
