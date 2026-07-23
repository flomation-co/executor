// Package oracle_identity_customer_secret_key_list lists the customer secret keys
// belonging to an IAM user (the S3-compatible secret is never returned by list).
package oracle_identity_customer_secret_key_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List Customer Secret Keys"
	Description  = "List the customer secret keys belonging to an Oracle Cloud IAM user (used with Object Storage's S3-compatible API) — their OCID, display name, lifecycle state and creation time. The secret key itself is never returned by a list (only on create)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the user picker)"},
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… whose customer secret keys to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "customer_secret_keys", Type: core.ConnectionTypeObject, Label: "Customer Secret Keys"},
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
	// ListCustomerSecretKeys takes a user path and returns the full set in one call (the
	// request carries no page cursor), so there is no pagination loop — an opc-next-page
	// header simply means the service returned a partial list we cannot page past.
	resp, err := client.ListCustomerSecretKeys(iam.Context(), identity.ListCustomerSecretKeysRequest{UserId: &userID})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	// CustomerSecretKeySummary has no shared summariser — build the map inline from its
	// fields. The secret key value is never part of a summary (returned only on create).
	out := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		k := &resp.Items[i]
		out = append(out, map[string]interface{}{
			"id":              iam.Str(k.Id),
			"display_name":    iam.Str(k.DisplayName),
			"user_id":         iam.Str(k.UserId),
			"lifecycle_state": string(k.LifecycleState),
			"time_created":    iam.FormatTime(k.TimeCreated),
			"time_expires":    iam.FormatTime(k.TimeExpires),
		})
	}
	truncated := resp.OpcNextPage != nil && *resp.OpcNextPage != ""
	return iam.Result(fmt.Sprintf("Found %d customer secret key(s)", len(out)), map[string]interface{}{
		"customer_secret_keys": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
