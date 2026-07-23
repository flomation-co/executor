// Package oracle_identity_customer_secret_key_create creates a customer secret key
// (S3-compatible access/secret pair) for a user. The secret key is returned ONCE, on
// create, and never again.
package oracle_identity_customer_secret_key_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create Customer Secret Key"
	Description  = "Create a customer secret key for an Oracle Cloud IAM user — an access-key/secret-key pair for Object Storage's Amazon S3-compatible API. The secret key value is returned ONCE here and never shown again, so capture it now."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… the secret key belongs to", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for this secret key (need not be unique)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "customer_secret_key", Type: core.ConnectionTypeObject, Label: "Customer Secret Key"},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Secret Key (shown once)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Access Key ID"},
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
	displayName, err := iam.RequiredString("display_name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateCustomerSecretKey(iam.Context(), identity.CreateCustomerSecretKeyRequest{
		UserId:                         &userID,
		CreateCustomerSecretKeyDetails: identity.CreateCustomerSecretKeyDetails{DisplayName: &displayName},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	// No summariser for CustomerSecretKey — build the map inline, deliberately omitting
	// the secret Key (surfaced separately, shown once).
	secretKey := map[string]interface{}{
		"id":              iam.Str(resp.CustomerSecretKey.Id),
		"user_id":         iam.Str(resp.CustomerSecretKey.UserId),
		"display_name":    iam.Str(resp.CustomerSecretKey.DisplayName),
		"lifecycle_state": string(resp.CustomerSecretKey.LifecycleState),
		"time_created":    iam.FormatTime(resp.CustomerSecretKey.TimeCreated),
		"time_expires":    iam.FormatTime(resp.CustomerSecretKey.TimeExpires),
	}
	return iam.Result(fmt.Sprintf("Created customer secret key %q — capture the secret key value now, it is shown only once", displayName), map[string]interface{}{
		"customer_secret_key": secretKey, "key": iam.Str(resp.CustomerSecretKey.Key), "id": secretKey["id"],
	}), nil
}
