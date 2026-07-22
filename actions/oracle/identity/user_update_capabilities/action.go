// Package oracle_identity_user_update_capabilities toggles an IAM user's credential capabilities.
package oracle_identity_user_update_capabilities

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
	Name         = "OCI Identity: Update User Capabilities"
	Description  = "Enable or disable an Oracle Cloud IAM user's credential capabilities (console password, API keys, auth tokens, SMTP/DB/customer-secret/OAuth2 credentials). Only the switches you set are changed; the rest are left as-is."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+user"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the user picker)"},
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID (to update)", Placeholder: "ocid1.user.oc1..aaaa… of the user whose capabilities to change", Required: true},
	{Name: "can_use_console_password", Type: core.ConnectionTypeBoolean, Label: "Can Use Console Password", Placeholder: "Allow console login (leave unset to keep unchanged)"},
	{Name: "can_use_api_keys", Type: core.ConnectionTypeBoolean, Label: "Can Use API Keys", Placeholder: "Allow API signing keys (leave unset to keep unchanged)"},
	{Name: "can_use_auth_tokens", Type: core.ConnectionTypeBoolean, Label: "Can Use Auth Tokens", Placeholder: "Allow auth/SWIFT tokens (leave unset to keep unchanged)"},
	{Name: "can_use_smtp_credentials", Type: core.ConnectionTypeBoolean, Label: "Can Use SMTP Credentials", Placeholder: "Allow SMTP passwords (leave unset to keep unchanged)"},
	{Name: "can_use_db_credentials", Type: core.ConnectionTypeBoolean, Label: "Can Use DB Credentials", Placeholder: "Allow database passwords (leave unset to keep unchanged)"},
	{Name: "can_use_customer_secret_keys", Type: core.ConnectionTypeBoolean, Label: "Can Use Customer Secret Keys", Placeholder: "Allow SigV4 symmetric keys (leave unset to keep unchanged)"},
	{Name: "can_use_oauth2_client_credentials", Type: core.ConnectionTypeBoolean, Label: "Can Use OAuth2 Client Credentials", Placeholder: "Allow OAuth2 credentials/tokens (leave unset to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "User OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "target_user_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdateUserCapabilitiesDetails{}
	var changed []string

	// Only overlay a capability when the operator actually supplied the switch —
	// leaving a *bool nil tells OCI to leave that capability unchanged.
	set := func(name, label string, dst **bool) {
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

	set("can_use_console_password", "console password", &details.CanUseConsolePassword)
	set("can_use_api_keys", "API keys", &details.CanUseApiKeys)
	set("can_use_auth_tokens", "auth tokens", &details.CanUseAuthTokens)
	set("can_use_smtp_credentials", "SMTP credentials", &details.CanUseSmtpCredentials)
	set("can_use_db_credentials", "DB credentials", &details.CanUseDBCredentials)
	set("can_use_customer_secret_keys", "customer secret keys", &details.CanUseCustomerSecretKeys)
	set("can_use_oauth2_client_credentials", "OAuth2 client credentials", &details.CanUseOAuth2ClientCredentials)

	if len(changed) == 0 {
		return iam.ErrorResult("Set at least one capability switch to update — none were provided."), nil
	}

	resp, err := client.UpdateUserCapabilities(iam.Context(), identity.UpdateUserCapabilitiesRequest{
		UserId:                        &id,
		UpdateUserCapabilitiesDetails: details,
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	user := iam.SummariseUser(&resp.User)
	msg := fmt.Sprintf("Updated capabilities for user %q: %s", user["name"], strings.Join(changed, ", "))
	return iam.Result(msg, map[string]interface{}{"user": user, "id": user["id"]}), nil
}
