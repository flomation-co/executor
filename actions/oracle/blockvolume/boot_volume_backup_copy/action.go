// Package oracle_blockvolume_boot_volume_backup_copy copies a boot-volume backup to
// another region — the cross-region DR building block for boot volumes. The copy runs
// asynchronously and returns a work-request id.
package oracle_blockvolume_boot_volume_backup_copy

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
	Name         = "OCI Block Volumes: Copy Boot Volume Backup"
	Description  = "Copy an Oracle Cloud boot-volume backup to another region for cross-region DR. The copy runs asynchronously and returns a work-request id."
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
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "The SOURCE region, e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "boot_volume_backup_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume Backup OCID", Placeholder: "ocid1.bootvolumebackup.oc1..aaaa… (the backup to copy)", Required: true},
	{Name: "destination_region", Type: core.ConnectionTypeString, Label: "Destination Region", Placeholder: "The target region, e.g. uk-cardiff-1", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the copied backup (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Copied Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Copied Backup OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "boot_volume_backup_ocid")
	if errResult != nil {
		return errResult, nil
	}
	dest := bv.NormaliseRegion("destination_region", inputs)
	if dest == "" {
		return bv.ErrorResult("destination region is required"), nil
	}
	details := ocicore.CopyBootVolumeBackupDetails{DestinationRegion: &dest}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	resp, err := client.CopyBootVolumeBackup(bv.Context(), ocicore.CopyBootVolumeBackupRequest{
		BootVolumeBackupId:          &id,
		CopyBootVolumeBackupDetails: details,
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	backup := bv.SummariseBootVolumeBackup(&resp.BootVolumeBackup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Copying boot volume backup %s to %s", id, dest),
		"backup":          backup,
		"id":              backup["id"],
		"work_request_id": bv.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
