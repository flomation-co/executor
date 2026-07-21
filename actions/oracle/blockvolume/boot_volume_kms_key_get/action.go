// Package oracle_blockvolume_boot_volume_kms_key_get reads the Vault (KMS) master
// encryption key assigned to a boot volume.
package oracle_blockvolume_boot_volume_kms_key_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Get Boot Volume Encryption Key"
	Description  = "Read the Vault (KMS) master encryption key assigned to an Oracle Cloud boot volume — blank if the volume uses Oracle-managed encryption."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the boot volume picker)"},
	{Name: "boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume OCID", Placeholder: "ocid1.bootvolume.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key OCID"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Boot Volume OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "boot_volume_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetBootVolumeKmsKey(bv.Context(), ocicore.GetBootVolumeKmsKeyRequest{BootVolumeId: &id})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	kmsKeyID := bv.Str(resp.KmsKeyId)
	result := fmt.Sprintf("Boot volume %s uses Vault key %s", id, kmsKeyID)
	if kmsKeyID == "" {
		result = fmt.Sprintf("Boot volume %s uses Oracle-managed encryption (no Vault key)", id)
	}
	return map[string]interface{}{
		"tool_result": result,
		"kms_key_id":  kmsKeyID,
		"id":          id,
		"success":     true,
	}, nil
}
