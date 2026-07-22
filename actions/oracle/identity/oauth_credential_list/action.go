// Package oracle_identity_oauth_credential_list lists the OAuth 2.0 client credentials
// belonging to an IAM user (the client-secret value is never returned by a list — only on
// create).
package oracle_identity_oauth_credential_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List OAuth Client Credentials"
	Description  = "List the OAuth 2.0 client credentials belonging to an Oracle Cloud IAM user — their OCID, name, description, scopes, expiry and lifecycle state. The client-secret value is never returned by a list (only once on create)."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… whose OAuth client credentials to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "oauth_credentials", Type: core.ConnectionTypeObject, Label: "OAuth Client Credentials"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// summariseOAuthCredential builds the result map inline (no shared summariser) from the
// OAuth2ClientCredentialSummary's own fields — deliberately never the client secret, which
// this type does not even carry (it is only returned once, on create).
func summariseOAuthCredential(c *identity.OAuth2ClientCredentialSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              iam.Str(c.Id),
		"name":            iam.Str(c.Name),
		"description":     iam.Str(c.Description),
		"compartment_id":  iam.Str(c.CompartmentId),
		"user_id":         iam.Str(c.UserId),
		"scopes":          c.Scopes,
		"expires_on":      iam.FormatTime(c.ExpiresOn),
		"lifecycle_state": string(c.LifecycleState),
		"time_created":    iam.FormatTime(c.TimeCreated),
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, userID, errResult := iam.ResourceClient(inputs, "target_user_ocid")
	if errResult != nil {
		return errResult, nil
	}
	req := identity.ListOAuthClientCredentialsRequest{UserId: &userID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListOAuthClientCredentials(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseOAuthCredential(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return iam.Result(fmt.Sprintf("Found %d OAuth client credential(s)", len(out)), map[string]interface{}{
		"oauth_credentials": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
