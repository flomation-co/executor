// Package oracle_blockvolume_boot_volume_update updates a boot volume — its
// display name, size (resize up), performance tier (VPUs/GB) or freeform tags.
package oracle_blockvolume_boot_volume_update

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
	Name         = "OCI Block Volumes: Update Boot Volume"
	Description  = "Update an Oracle Cloud boot volume — rename it, resize it up, change its performance tier (VPUs/GB) or replace its freeform tags. Only the fields you supply are changed."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the boot-volume picker)"},
	{Name: "boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume OCID", Placeholder: "ocid1.bootvolume.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name (optional)"},
	{Name: "size_in_gbs", Type: core.ConnectionTypeString, Label: "Size (GB)", Placeholder: "New size — must be larger than the current size (optional)"},
	{Name: "vpus_per_gb", Type: core.ConnectionTypeString, Label: "Performance (VPUs/GB)", Placeholder: "10 (Balanced), 20 (Higher Perf), 30–120 (Ultra High) (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
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
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "boot_volume_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateBootVolumeDetails{FreeformTags: tags}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
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

	resp, err := client.UpdateBootVolume(bv.Context(), ocicore.UpdateBootVolumeRequest{
		BootVolumeId:            &id,
		UpdateBootVolumeDetails: details,
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	boot := bv.SummariseBootVolume(&resp.BootVolume)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated boot volume %q (%s)", boot["display_name"], boot["lifecycle_state"]),
		"boot_volume":     boot,
		"id":              boot["id"],
		"lifecycle_state": boot["lifecycle_state"],
		"success":         true,
	}, nil
}
