// Package oracle_blockvolume_volume_create provisions a block volume — a new empty
// volume (availability domain + size), a clone of an existing volume, or a restore
// from a backup.
package oracle_blockvolume_volume_create

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
	Name         = "OCI Block Volumes: Create Volume"
	Description  = "Provision an Oracle Cloud block volume — a new empty volume (availability domain + size in GB), a clone of an existing volume, or a restore from a backup. Optionally attach a backup policy at create time."
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (required unless cloning an existing volume)"},
	{Name: "size_in_gbs", Type: core.ConnectionTypeString, Label: "Size (GB)", Placeholder: "50–32768 for a new volume (default 50)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "vpus_per_gb", Type: core.ConnectionTypeString, Label: "Performance (VPUs/GB)", Placeholder: "0 (Lower Cost), 10 (Balanced, default), 20 or 30 (Higher Perf) (optional)"},
	{Name: "backup_policy_ocid", Type: core.ConnectionTypeString, Label: "Backup Policy OCID", Placeholder: "Assign a backup policy at create time (optional)"},
	{Name: "source_volume_ocid", Type: core.ConnectionTypeString, Label: "Clone From Volume OCID", Placeholder: "Clone from this volume instead of a new one (optional)"},
	{Name: "volume_backup_ocid", Type: core.ConnectionTypeString, Label: "Restore From Backup OCID", Placeholder: "Restore from this backup instead of a new one (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volume", Type: core.ConnectionTypeObject, Label: "Volume"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Volume OCID"},
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
	details := ocicore.CreateVolumeDetails{CompartmentId: &compartment, FreeformTags: tags}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(bv.OptionalString("availability_domain", inputs)); v != "" {
		details.AvailabilityDomain = &v
	}
	if v := strings.TrimSpace(bv.OptionalString("backup_policy_ocid", inputs)); v != "" {
		details.BackupPolicyId = &v
	}
	if vpus, ok, err := bv.OptionalInt64("vpus_per_gb", inputs); err != nil {
		return bv.ErrorResult(err.Error()), nil
	} else if ok {
		details.VpusPerGB = &vpus
	}
	// Source precedence: clone > restore-from-backup > new (AD + size).
	if src := strings.TrimSpace(bv.OptionalString("source_volume_ocid", inputs)); src != "" {
		details.SourceDetails = ocicore.VolumeSourceFromVolumeDetails{Id: &src}
	} else if bk := strings.TrimSpace(bv.OptionalString("volume_backup_ocid", inputs)); bk != "" {
		// A backup can be restored into any AD in the region, so OCI can't infer one
		// (unlike a clone, which inherits the source volume's AD) — require it.
		if details.AvailabilityDomain == nil {
			return bv.ErrorResult("availability domain is required when restoring from a backup (it is only omissible when cloning an existing volume)"), nil
		}
		details.VolumeBackupId = &bk
	} else {
		if details.AvailabilityDomain == nil {
			return bv.ErrorResult("availability domain is required for a new volume (or supply a source volume / backup to clone or restore)"), nil
		}
		size := int64(50)
		if s, ok, err := bv.OptionalInt64("size_in_gbs", inputs); err != nil {
			return bv.ErrorResult(err.Error()), nil
		} else if ok {
			size = s
		}
		details.SizeInGBs = &size
	}

	client, err := auth.BlockstorageClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateVolume(bv.Context(), ocicore.CreateVolumeRequest{CreateVolumeDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	vol := bv.SummariseVolume(&resp.Volume)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created volume %q (%s)", vol["display_name"], vol["lifecycle_state"]),
		"volume":          vol,
		"id":              vol["id"],
		"lifecycle_state": vol["lifecycle_state"],
		"success":         true,
	}, nil
}
