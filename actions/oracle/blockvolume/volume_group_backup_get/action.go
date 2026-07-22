// Package oracle_blockvolume_volume_group_backup_get reads one volume-group backup by OCID.
package oracle_blockvolume_volume_group_backup_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Get Volume Group Backup"
	Description  = "Fetch a single Oracle Cloud volume-group backup by OCID — its type, source, size, member volume backups and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+floppy-disk"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the backup picker)"},
	{Name: "volume_group_backup_ocid", Type: core.ConnectionTypeString, Label: "Volume Group Backup OCID", Placeholder: "ocid1.volumegroupbackup.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volume_group_backup", Type: core.ConnectionTypeObject, Label: "Volume Group Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Volume Group Backup OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_group_backup_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetVolumeGroupBackup(bv.Context(), ocicore.GetVolumeGroupBackupRequest{VolumeGroupBackupId: &id})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	backup := bv.SummariseVolumeGroupBackup(&resp.VolumeGroupBackup)
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Volume group backup %q is %s", backup["display_name"], backup["lifecycle_state"]),
		"volume_group_backup": backup,
		"id":                  backup["id"],
		"lifecycle_state":     backup["lifecycle_state"],
		"success":             true,
	}, nil
}
