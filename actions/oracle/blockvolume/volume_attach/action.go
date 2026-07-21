// Package oracle_blockvolume_volume_attach attaches a block volume to a compute
// instance. This action spans two OCI service clients — the attach call itself lives
// on the Compute client, keyed by the instance OCID (path) with the volume OCID in
// the body. A paravirtualized attachment is the sane default (no in-guest iSCSI
// setup); iSCSI and emulated modes are out of scope for the operator surface.
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
	Description  = "Attach an Oracle Cloud block volume to a compute instance (paravirtualized). Optionally set the device path, read-only and shareable flags."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plug"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the instance & volume pickers)"},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID", Placeholder: "ocid1.instance.oc1..aaaa… (the compute instance to attach to)", Required: true},
	{Name: "volume_ocid", Type: core.ConnectionTypeString, Label: "Volume OCID", Placeholder: "ocid1.volume.oc1..aaaa… (the block volume to attach)", Required: true},
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
	details := ocicore.AttachParavirtualizedVolumeDetails{InstanceId: &instanceID, VolumeId: &volumeID}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(bv.OptionalString("device", inputs)); v != "" {
		details.Device = &v
	}
	if bv.BoolWasSet("is_read_only", inputs) {
		ro := bv.OptionalBool("is_read_only", inputs, false)
		details.IsReadOnly = &ro
	}
	if bv.BoolWasSet("is_shareable", inputs) {
		sh := bv.OptionalBool("is_shareable", inputs, false)
		details.IsShareable = &sh
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
