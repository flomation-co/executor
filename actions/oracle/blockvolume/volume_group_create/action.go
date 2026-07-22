// Package oracle_blockvolume_volume_group_create provisions a volume group — a set
// of block volumes managed as one — built from a list of existing volumes, cloned
// from another volume group, or restored from a volume group backup.
package oracle_blockvolume_volume_group_create

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
	Name         = "OCI Block Volumes: Create Volume Group"
	Description  = "Create an Oracle Cloud volume group — a set of block volumes managed as one — built from a list of existing volumes, cloned from another volume group, or restored from a volume group backup."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+layer-group"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1", Required: true},
	{Name: "volume_ocids", Type: core.ConnectionTypeString, Label: "Volume OCIDs", Placeholder: "Comma-separated volume OCIDs to group together"},
	{Name: "source_volume_group_ocid", Type: core.ConnectionTypeString, Label: "Clone From Volume Group OCID", Placeholder: "Clone from this volume group instead (optional)"},
	{Name: "volume_group_backup_ocid", Type: core.ConnectionTypeString, Label: "Restore From Volume Group Backup OCID", Placeholder: "Restore from this volume group backup instead (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volume_group", Type: core.ConnectionTypeObject, Label: "Volume Group"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Volume Group OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := bv.GetAuth(inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	ad, err := bv.RequiredString("availability_domain", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreateVolumeGroupDetails{
		CompartmentId:      &compartment,
		AvailabilityDomain: &ad,
		FreeformTags:       tags,
	}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	// Source precedence: existing volumes > clone volume group > restore-from-backup.
	if vols := bv.InputStrings("volume_ocids", inputs); len(vols) > 0 {
		details.SourceDetails = ocicore.VolumeGroupSourceFromVolumesDetails{VolumeIds: vols}
	} else if grp := strings.TrimSpace(bv.OptionalString("source_volume_group_ocid", inputs)); grp != "" {
		details.SourceDetails = ocicore.VolumeGroupSourceFromVolumeGroupDetails{VolumeGroupId: &grp}
	} else if bk := strings.TrimSpace(bv.OptionalString("volume_group_backup_ocid", inputs)); bk != "" {
		details.SourceDetails = ocicore.VolumeGroupSourceFromVolumeGroupBackupDetails{VolumeGroupBackupId: &bk}
	} else {
		return bv.ErrorResult("a volume group source is required — supply volume OCIDs to group, a source volume group to clone, or a volume group backup to restore"), nil
	}

	client, err := auth.BlockstorageClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateVolumeGroup(bv.Context(), ocicore.CreateVolumeGroupRequest{CreateVolumeGroupDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	grp := bv.SummariseVolumeGroup(&resp.VolumeGroup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created volume group %q (%s)", grp["display_name"], grp["lifecycle_state"]),
		"volume_group":    grp,
		"id":              grp["id"],
		"lifecycle_state": grp["lifecycle_state"],
		"success":         true,
	}, nil
}
