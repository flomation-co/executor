// Package oracle_blockvolume_volume_group_update updates an Oracle Cloud volume
// group — its display name, freeform tags, and (with REPLACE semantics) the set of
// member volumes.
package oracle_blockvolume_volume_group_update

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
	Name         = "OCI Block Volumes: Update Volume Group"
	Description  = "Update an Oracle Cloud volume group — its display name, freeform tags, and its set of member volumes. Note: supplying member volumes REPLACES the group's entire membership (it is not additive)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+layer-group"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the volume group picker)"},
	{Name: "volume_group_ocid", Type: core.ConnectionTypeString, Label: "Volume Group OCID", Placeholder: "ocid1.volumegroup.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "volume_ocids", Type: core.ConnectionTypeString, Label: "Member Volume OCIDs (comma-separated)", Placeholder: "ocid1.volume.oc1..aaa,ocid1.volume.oc1..bbb — REPLACES the whole member set (optional)"},
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
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_group_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := bv.FreeformTags("tags", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}

	details := ocicore.UpdateVolumeGroupDetails{}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	// REPLACE semantics: OCI overwrites the group's entire member set with VolumeIds,
	// so only send it when the operator supplied one — leaving it blank keeps the
	// existing membership untouched (it is not additive).
	if ids := bv.InputStrings("volume_ocids", inputs); len(ids) > 0 {
		details.VolumeIds = ids
	}

	resp, err := client.UpdateVolumeGroup(bv.Context(), ocicore.UpdateVolumeGroupRequest{
		VolumeGroupId:            &id,
		UpdateVolumeGroupDetails: details,
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	g := bv.SummariseVolumeGroup(&resp.VolumeGroup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated volume group %q (%s)", g["display_name"], g["lifecycle_state"]),
		"volume_group":    g,
		"id":              g["id"],
		"lifecycle_state": g["lifecycle_state"],
		"success":         true,
	}, nil
}
