// Package oracle_blockvolume_volume_kms_key_update assigns or rotates the
// customer-managed Vault (KMS) master encryption key on one block volume.
package oracle_blockvolume_volume_kms_key_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Set Volume Encryption Key"
	Description  = "Assign or rotate the customer-managed Vault (KMS) master encryption key protecting an Oracle Cloud block volume."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the volume picker)"},
	{Name: "volume_ocid", Type: core.ConnectionTypeString, Label: "Volume OCID", Placeholder: "ocid1.volume.oc1..aaaa…", Required: true},
	{Name: "kms_key_ocid", Type: core.ConnectionTypeString, Label: "KMS Key OCID", Placeholder: "ocid1.key.oc1..aaaa… (same OCID as before regenerates the volume encryption key)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key OCID"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Volume OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_ocid")
	if errResult != nil {
		return errResult, nil
	}
	key, err := bv.RequiredString("kms_key_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	resp, err := client.UpdateVolumeKmsKey(bv.Context(), ocicore.UpdateVolumeKmsKeyRequest{
		VolumeId: &id,
		UpdateVolumeKmsKeyDetails: ocicore.UpdateVolumeKmsKeyDetails{
			KmsKeyId: &key,
		},
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	kmsKeyID := bv.Str(resp.VolumeKmsKey.KmsKeyId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Set encryption key %s on volume %s", kmsKeyID, id),
		"kms_key_id":  kmsKeyID,
		"id":          id,
		"success":     true,
	}, nil
}
