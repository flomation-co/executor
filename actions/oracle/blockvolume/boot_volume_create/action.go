// Package oracle_blockvolume_boot_volume_create provisions a boot volume. Unlike a
// data volume a boot volume is never blank — it must be seeded from a source: a clone
// of an existing boot volume, a restore from a boot-volume backup, or from a
// boot-volume replica.
package oracle_blockvolume_boot_volume_create

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
	Name         = "OCI Block Volumes: Create Boot Volume"
	Description  = "Provision an Oracle Cloud boot volume from a source — a clone of an existing boot volume, a restore from a boot-volume backup, or from a boot-volume replica. Boot volumes are never created blank."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+hard-drive"
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
	{Name: "source_boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Clone From Boot Volume OCID", Placeholder: "Clone from this boot volume (optional — one source is required)"},
	{Name: "boot_volume_backup_ocid", Type: core.ConnectionTypeString, Label: "Restore From Backup OCID", Placeholder: "Restore from this boot-volume backup (optional — one source is required)"},
	{Name: "boot_volume_replica_ocid", Type: core.ConnectionTypeString, Label: "Create From Replica OCID", Placeholder: "Create from this boot-volume replica (optional — one source is required)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (optional — omissible when cloning)"},
	{Name: "size_in_gbs", Type: core.ConnectionTypeString, Label: "Size (GB)", Placeholder: "Boot-volume size in GB (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "vpus_per_gb", Type: core.ConnectionTypeString, Label: "Performance (VPUs/GB)", Placeholder: "10 (Balanced, default), 20 or 30+ (Higher Perf) (optional)"},
	{Name: "kms_key_ocid", Type: core.ConnectionTypeString, Label: "KMS Key OCID", Placeholder: "Vault master encryption key OCID (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "boot_volume", Type: core.ConnectionTypeObject, Label: "Boot Volume"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Boot Volume OCID"},
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
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.CreateBootVolumeDetails{CompartmentId: &compartment, FreeformTags: tags}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(bv.OptionalString("availability_domain", inputs)); v != "" {
		details.AvailabilityDomain = &v
	}
	if v := strings.TrimSpace(bv.OptionalString("kms_key_ocid", inputs)); v != "" {
		details.KmsKeyId = &v
	}
	if size, ok, err := bv.OptionalInt64("size_in_gbs", inputs); err != nil {
		return bv.ErrorResult(err.Error()), nil
	} else if ok {
		details.SizeInGBs = &size
	}
	if vpus, ok, err := bv.OptionalInt64("vpus_per_gb", inputs); err != nil {
		return bv.ErrorResult(err.Error()), nil
	} else if ok {
		details.VpusPerGB = &vpus
	}
	// Boot volumes are never blank — a source is mandatory.
	// Source precedence: clone > restore-from-backup > from-replica.
	if src := strings.TrimSpace(bv.OptionalString("source_boot_volume_ocid", inputs)); src != "" {
		details.SourceDetails = ocicore.BootVolumeSourceFromBootVolumeDetails{Id: &src}
	} else if bk := strings.TrimSpace(bv.OptionalString("boot_volume_backup_ocid", inputs)); bk != "" {
		details.SourceDetails = ocicore.BootVolumeSourceFromBootVolumeBackupDetails{Id: &bk}
	} else if rep := strings.TrimSpace(bv.OptionalString("boot_volume_replica_ocid", inputs)); rep != "" {
		details.SourceDetails = ocicore.BootVolumeSourceFromBootVolumeReplicaDetails{Id: &rep}
	} else {
		return bv.ErrorResult("a source is required to create a boot volume — supply a boot volume to clone, a boot-volume backup to restore, or a boot-volume replica"), nil
	}

	client, err := auth.BlockstorageClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateBootVolume(bv.Context(), ocicore.CreateBootVolumeRequest{CreateBootVolumeDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	vol := bv.SummariseBootVolume(&resp.BootVolume)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created boot volume %q (%s)", vol["display_name"], vol["lifecycle_state"]),
		"boot_volume":     vol,
		"id":              vol["id"],
		"lifecycle_state": vol["lifecycle_state"],
		"success":         true,
	}, nil
}
