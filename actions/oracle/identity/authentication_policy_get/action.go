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
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
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
