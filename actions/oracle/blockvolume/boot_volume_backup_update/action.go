// Package oracle_blockvolume_boot_volume_backup_update renames a boot-volume backup
// and/or replaces its freeform tags.
package oracle_blockvolume_boot_volume_backup_update

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
	Name         = "OCI Block Volumes: Update Boot Volume Backup"
	Description  = "Update an Oracle Cloud boot-volume backup — change its display name and/or freeform tags."
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
	{Name: "boot_volume_backup_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume Backup OCID", Placeholder: "ocid1.bootvolumebackup.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name for the backup (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "boot_volume_backup", Type: core.ConnectionTypeObject, Label: "Boot Volume Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Boot Volume Backup OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "boot_volume_backup_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateBootVolumeBackupDetails{}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateBootVolumeBackup(bv.Context(), ocicore.UpdateBootVolumeBackupRequest{
		BootVolumeBackupId:            &id,
		UpdateBootVolumeBackupDetails: details,
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	backup := bv.SummariseBootVolumeBackup(&resp.BootVolumeBackup)
	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Updated boot volume backup %q (%s)", backup["display_name"], backup["lifecycle_state"]),
		"boot_volume_backup": backup,
		"id":                 backup["id"],
		"lifecycle_state":    backup["lifecycle_state"],
		"success":            true,
	}, nil
}
