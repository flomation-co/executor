// Package oracle_blockvolume_volume_group_backup_copy copies a volume-group backup
// to another region — the cross-region DR building block for a whole volume group.
// The copy runs asynchronously and returns the newly created backup as it begins to
// hydrate in the destination region.
package oracle_blockvolume_volume_group_backup_copy

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Copy Volume Group Backup"
	Description  = "Copy an Oracle Cloud volume-group backup to another region for cross-region DR. The copy runs asynchronously and returns the copied volume-group backup."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+floppy-disk"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "The SOURCE region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "volume_group_backup_ocid", Type: core.ConnectionTypeString, Label: "Volume Group Backup OCID", Placeholder: "ocid1.volumegroupbackup.oc1..aaaa… (the backup to copy)", Required: true},
	{Name: "destination_region", Type: core.ConnectionTypeString, Label: "Destination Region", Placeholder: "The target region, e.g. uk-cardiff-1", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the copied volume-group backup (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Copied Volume Group Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Copied Volume Group Backup OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_group_backup_ocid")
	if errResult != nil {
		return errResult, nil
	}
	dest := bv.NormaliseRegion("destination_region", inputs)
	if dest == "" {
		return bv.ErrorResult("destination region is required"), nil
	}
	details := ocicore.CopyVolumeGroupBackupDetails{DestinationRegion: &dest}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	resp, err := client.CopyVolumeGroupBackup(bv.Context(), ocicore.CopyVolumeGroupBackupRequest{
		VolumeGroupBackupId:          &id,
		CopyVolumeGroupBackupDetails: details,
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	backup := bv.SummariseVolumeGroupBackup(&resp.VolumeGroupBackup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Copying volume group backup %s to %s", id, dest),
		"backup":          backup,
		"id":              backup["id"],
		"lifecycle_state": backup["lifecycle_state"],
		"success":         true,
	}, nil
}
