// Package oracle_identity_mfa_totp_list lists the MFA TOTP devices registered for an IAM user.
package oracle_identity_mfa_totp_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List MFA TOTP Devices"
	Description  = "List the MFA TOTP (authenticator-app) devices registered for an Oracle Cloud IAM user — each device's OCID, activation flag and lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "target_user_ocid", Type: core.ConnectionTypeString, Label: "User OCID (to read)", Placeholder: "ocid1.user.oc1..aaaa… whose MFA TOTP devices to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mfa_totp_devices", Type: core.ConnectionTypeObject, Label: "MFA TOTP Devices"},
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
	req := identity.ListMfaTotpDevicesRequest{UserId: &userID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListMfaTotpDevices(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseMfaTotpDevice(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return iam.Result(fmt.Sprintf("Found %d MFA TOTP device(s)", len(out)), map[string]interface{}{
		"mfa_totp_devices": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}

// summariseMfaTotpDevice flattens an MfaTotpDeviceSummary (no shared summariser exists for
// this type). inactive_status is only meaningful when lifecycle_state is INACTIVE.
func summariseMfaTotpDevice(d *identity.MfaTotpDeviceSummary) map[string]interface{} {
	m := map[string]interface{}{
		"id":              iam.Str(d.Id),
		"user_id":         iam.Str(d.UserId),
		"lifecycle_state": string(d.LifecycleState),
		"is_activated":    d.IsActivated != nil && *d.IsActivated,
		"time_created":    iam.FormatTime(d.TimeCreated),
		"time_expires":    iam.FormatTime(d.TimeExpires),
	}
	if d.InactiveStatus != nil {
		m["inactive_status"] = *d.InactiveStatus
	}
	return m
}
