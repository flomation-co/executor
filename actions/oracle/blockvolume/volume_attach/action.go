// Package oracle_blockvolume_volume_attach attaches a block volume to a compute
// instance. This action spans two OCI service clients — the attach call itself lives
// on the Compute client, keyed by the instance OCID (path) with the volume OCID in
// the body. Paravirtualized is the default (no in-guest setup, works on VM shapes);
// iSCSI is required for bare-metal instances and emulated is available for legacy
// images, so the attachment type is operator-selectable.
package oracle_blockvolume_volume_attach

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
	Name         = "OCI Block Volumes: Attach Volume"
	Description  = "Attach an Oracle Cloud block volume to a compute instance. Defaults to a paravirtualized attachment (VM shapes); choose iSCSI for bare-metal instances or emulated for legacy images. Optionally set the device path, read-only and shareable flags."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plug"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the instance & volume pickers)"},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID", Placeholder: "ocid1.instance.oc1..aaaa… (the compute instance to attach to)", Required: true},
	{Name: "volume_ocid", Type: core.ConnectionTypeString, Label: "Volume OCID", Placeholder: "ocid1.volume.oc1..aaaa… (the block volume to attach)", Required: true},
	{Name: "attachment_type", Type: core.ConnectionTypeString, Label: "Attachment Type", Placeholder: "Paravirtualized (default, VM shapes), iSCSI (required for bare-metal), or Emulated", Options: []core.ConnectionOption{
		{Name: "Paravirtualized", Value: "paravirtualized"},
		{Name: "iSCSI", Value: "iscsi"},
		{Name: "Emulated", Value: "emulated"},
	}},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the attachment (optional)"},
	{Name: "device", Type: core.ConnectionTypeString, Label: "Device Path", Placeholder: "e.g. /dev/oracleoci/oraclevdb — pin the in-guest device name (optional)"},
	{Name: "is_read_only", Type: core.ConnectionTypeBoolean, Label: "Read Only", Placeholder: "Attach the volume read-only (optional)"},
	{Name: "is_shareable", Type: core.ConnectionTypeBoolean, Label: "Shareable", Placeholder: "Allow the volume to be attached to multiple instances (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment", Type: core.ConnectionTypeObject, Label: "Volume Attachment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, instanceID, errResult := bv.ComputeResourceClient(inputs, "instance_ocid")
	if errResult != nil {
		return errResult, nil
	}
	volumeID, err := bv.RequiredString("volume_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	// Fields common to all three attachment types; nil when the operator left them blank.
	strPtr := func(name string) *string {
		if v := strings.TrimSpace(bv.OptionalString(name, inputs)); v != "" {
			return &v
		}
		return nil
	}
	boolPtr := func(name string) *bool {
		if bv.BoolWasSet(name, inputs) {
			b := bv.OptionalBool(name, inputs, false)
			return &b
		}
		return nil
	}
	displayName, device := strPtr("display_name"), strPtr("device")
	readOnly, shareable := boolPtr("is_read_only"), boolPtr("is_shareable")

	// All three concrete detail types implement the AttachVolumeDetails interface and
	// share these fields; paravirtualized is the default for VM shapes, iSCSI is
	// required for bare-metal instances, emulated is for legacy images.
	var details ocicore.AttachVolumeDetails
	switch strings.ToLower(strings.TrimSpace(bv.OptionalString("attachment_type", inputs))) {
	case "", "paravirtualized":
		details = ocicore.AttachParavirtualizedVolumeDetails{InstanceId: &instanceID, VolumeId: &volumeID, DisplayName: displayName, Device: device, IsReadOnly: readOnly, IsShareable: shareable}
	case "iscsi":
		details = ocicore.AttachIScsiVolumeDetails{InstanceId: &instanceID, VolumeId: &volumeID, DisplayName: displayName, Device: device, IsReadOnly: readOnly, IsShareable: shareable}
	case "emulated":
		details = ocicore.AttachEmulatedVolumeDetails{InstanceId: &instanceID, VolumeId: &volumeID, DisplayName: displayName, Device: device, IsReadOnly: readOnly, IsShareable: shareable}
	default:
		return bv.ErrorResult(fmt.Sprintf("attachment type %q is not valid — expected paravirtualized, iscsi or emulated", bv.OptionalString("attachment_type", inputs))), nil
	}
	resp, err := client.AttachVolume(bv.Context(), ocicore.AttachVolumeRequest{AttachVolumeDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	att := bv.SummariseVolumeAttachment(resp.VolumeAttachment)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Attaching volume %s to instance %s (%s)", volumeID, instanceID, att["lifecycle_state"]),
		"attachment":      att,
		"id":              att["id"],
		"lifecycle_state": att["lifecycle_state"],
		"success":         true,
	}, nil
}
