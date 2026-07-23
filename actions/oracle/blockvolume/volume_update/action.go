// Package oracle_blockvolume_volume_update updates a block volume in place — its
// display name, performance tier (VPUs/GB), size (expand only) and tags.
package oracle_blockvolume_volume_update

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
	Name         = "OCI Block Volumes: Update Volume"
	Description  = "Update an Oracle Cloud block volume in place — rename it, change its performance tier (VPUs/GB), expand its size in GB, or replace its freeform tags."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+hard-drive"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the volume picker)"},
	{Name: "volume_ocid", Type: core.ConnectionTypeString, Label: "Volume OCID", Placeholder: "ocid1.volume.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "vpus_per_gb", Type: core.ConnectionTypeString, Label: "Performance (VPUs/GB)", Placeholder: "0 (Lower Cost), 10 (Balanced), 20 or 30+ (Higher Perf) (optional)"},
	{Name: "size_in_gbs", Type: core.ConnectionTypeString, Label: "Size (GB)", Placeholder: "New size — must be LARGER than the current size (expand only, optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
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
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := ocicore.UpdateVolumeDetails{}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if vpus, ok, err := bv.OptionalInt64("vpus_per_gb", inputs); err != nil {
		return bv.ErrorResult(err.Error()), nil
	} else if ok {
		details.VpusPerGB = &vpus
	}
	if size, ok, err := bv.OptionalInt64("size_in_gbs", inputs); err != nil {
		return bv.ErrorResult(err.Error()), nil
	} else if ok {
		details.SizeInGBs = &size
	}
	if tags, err := bv.FreeformTags("tags", inputs); err != nil {
		return bv.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateVolume(bv.Context(), ocicore.UpdateVolumeRequest{VolumeId: &id, UpdateVolumeDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	vol := bv.SummariseVolume(&resp.Volume)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated volume %q (%s)", vol["display_name"], vol["lifecycle_state"]),
		"volume":          vol,
		"id":              vol["id"],
		"lifecycle_state": vol["lifecycle_state"],
		"success":         true,
	}, nil
}
