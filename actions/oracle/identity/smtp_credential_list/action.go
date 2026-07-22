// Package oracle_identity_smtp_credential_list lists the SMTP credentials belonging to an
// IAM user (the credential password is never returned by a list — only once on create).
package oracle_identity_smtp_credential_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List SMTP Credentials"
	Description  = "List the SMTP credentials belonging to an Oracle Cloud IAM user — their OCID, SMTP username, description and lifecycle state. The credential password is never returned by a list (only once on create)."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… whose SMTP credentials to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "smtp_credentials", Type: core.ConnectionTypeObject, Label: "SMTP Credentials"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, userID, errResult := iam.ResourceClient(inputs, "target_user_ocid")
	if errResult != nil {
		return errResult, nil
	}
	// ListSmtpCredentials takes a user path and returns the full set in one call (the
	// request carries no page cursor), so there is no pagination loop — an opc-next-page
	// header simply means the service returned a partial list we cannot page past.
	resp, err := client.ListSmtpCredentials(iam.Context(), identity.ListSmtpCredentialsRequest{UserId: &userID})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	out := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		c := &resp.Items[i]
		out = append(out, map[string]interface{}{
			"id":              iam.Str(c.Id),
			"username":        iam.Str(c.Username),
			"description":     iam.Str(c.Description),
			"lifecycle_state": string(c.LifecycleState),
			"time_created":    iam.FormatTime(c.TimeCreated),
		})
	}
	truncated := resp.OpcNextPage != nil && *resp.OpcNextPage != ""
	return iam.Result(fmt.Sprintf("Found %d SMTP credential(s)", len(out)), map[string]interface{}{
		"smtp_credentials": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
