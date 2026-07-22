// Package oracle_identity_oauth_credential_create creates an OAuth 2.0 client credential
// for a user. The generated client-secret password is returned ONCE, on create, and never
// again.
package oracle_identity_oauth_credential_create

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
	Name         = "OCI Identity: Create OAuth Client Credential"
	Description  = "Create an OAuth 2.0 client credential for an Oracle Cloud IAM user — a client-id/secret pair for the OAuth client-credentials grant. The secret (password) is returned ONCE here and never shown again, so capture it now."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the OAuth credential belongs to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A label to tell this credential apart", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this OAuth credential is for", Required: true},
	{Name: "scopes", Type: core.ConnectionTypeText, Label: "Scopes", Placeholder: "One scope per line, as `audience, scope` — e.g. urn:oracle:db::id::ocid1.autonomousdatabase…, urn:opc:resource:consumer::all", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "oauth_credential", Type: core.ConnectionTypeObject, Label: "OAuth Credential"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "Client Secret / Password (shown once)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "OAuth Credential OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	userID, err := iam.RequiredString("target_user_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	name, err := iam.RequiredString("name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	description, err := iam.RequiredString("description", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	lines := iam.InputLines("scopes", inputs)
	if len(lines) == 0 {
		return iam.ErrorResult("at least one scope is required — one per line, as `audience, scope`"), nil
	}
	var scopes []identity.FullyQualifiedScope
	for _, line := range lines {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			return iam.ErrorResult(fmt.Sprintf("scope %q must be `audience, scope` (audience and scope separated by a comma)", line)), nil
		}
		audience := strings.TrimSpace(parts[0])
		scope := strings.TrimSpace(parts[1])
		if audience == "" || scope == "" {
			return iam.ErrorResult(fmt.Sprintf("scope %q must give both an audience and a scope", line)), nil
		}
		scopes = append(scopes, identity.FullyQualifiedScope{Audience: &audience, Scope: &scope})
	}

	resp, err := client.CreateOAuthClientCredential(iam.Context(), identity.CreateOAuthClientCredentialRequest{
		UserId: &userID,
		CreateOAuth2ClientCredentialDetails: identity.CreateOAuth2ClientCredentialDetails{
			Name:        &name,
			Description: &description,
			Scopes:      scopes,
		},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	cred := summariseOAuthCredential(&resp.OAuth2ClientCredential)
	return iam.Result(fmt.Sprintf("Created OAuth client credential %q — capture the client secret (password) now, it is shown only once", name), map[string]interface{}{
		"oauth_credential": cred,
		"password":         iam.Str(resp.OAuth2ClientCredential.Password),
		"id":               cred["id"],
	}), nil
}

// summariseOAuthCredential shapes an OAuth2ClientCredential into a result map (there is no
// shared summariser for this type). The password is deliberately omitted here — it is
// surfaced separately, once, on create.
func summariseOAuthCredential(c *identity.OAuth2ClientCredential) map[string]interface{} {
	scopes := make([]map[string]interface{}, 0, len(c.Scopes))
	for _, s := range c.Scopes {
		scopes = append(scopes, map[string]interface{}{
			"audience": iam.Str(s.Audience),
			"scope":    iam.Str(s.Scope),
		})
	}
	return map[string]interface{}{
		"id":              iam.Str(c.Id),
		"name":            iam.Str(c.Name),
		"description":     iam.Str(c.Description),
		"user_id":         iam.Str(c.UserId),
		"compartment_id":  iam.Str(c.CompartmentId),
		"scopes":          scopes,
		"lifecycle_state": string(c.LifecycleState),
		"time_created":    iam.FormatTime(c.TimeCreated),
		"expires_on":      iam.FormatTime(c.ExpiresOn),
	}
}
