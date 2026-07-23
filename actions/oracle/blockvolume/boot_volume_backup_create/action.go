// Package oracle_blockvolume_boot_volume_backup_create takes a manual backup of a
// boot volume — a full or incremental point-in-time copy.
package oracle_blockvolume_boot_volume_backup_create

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
	Name         = "OCI Block Volumes: Create Boot Volume Backup"
	Description  = "Take a manual point-in-time backup of an Oracle Cloud boot volume — full (default) or incremental."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+floppy-disk"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the boot volume picker)"},
	{Name: "boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume OCID", Placeholder: "ocid1.bootvolume.oc1..aaaa… (the boot volume to back up)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the backup (optional)"},
	{Name: "backup_type", Type: core.ConnectionTypeString, Label: "Backup Type", Placeholder: "FULL (default) or INCREMENTAL", Options: []core.ConnectionOption{
		{Name: "Full", Value: "FULL"},
		{Name: "Incremental", Value: "INCREMENTAL"},
	}},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Boot Volume Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Backup OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, bootVolumeID, errResult := bv.VolumeResourceClient(inputs, "boot_volume_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreateBootVolumeBackupDetails{BootVolumeId: &bootVolumeID, FreeformTags: tags}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	switch strings.ToUpper(strings.TrimSpace(bv.OptionalString("backup_type", inputs))) {
	case "INCREMENTAL":
		details.Type = ocicore.CreateBootVolumeBackupDetailsTypeIncremental
	case "", "FULL":
		details.Type = ocicore.CreateBootVolumeBackupDetailsTypeFull
	default:
		return bv.ErrorResult("backup type must be FULL or INCREMENTAL"), nil
	}
	resp, err := client.CreateBootVolumeBackup(bv.Context(), ocicore.CreateBootVolumeBackupRequest{CreateBootVolumeBackupDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	backup := bv.SummariseBootVolumeBackup(&resp.BootVolumeBackup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Creating %s backup of boot volume %s (%s)", backup["type"], bootVolumeID, backup["lifecycle_state"]),
		"backup":          backup,
		"id":              backup["id"],
		"lifecycle_state": backup["lifecycle_state"],
		"success":         true,
	}, nil
}
