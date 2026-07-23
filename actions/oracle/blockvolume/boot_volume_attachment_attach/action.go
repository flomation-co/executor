// Package oracle_blockvolume_boot_volume_attachment_attach attaches a boot volume to
// a compute instance. Like the data-volume attach it spans two service clients — the
// attach call lives on the Compute client, keyed by the instance OCID (path) with the
// boot-volume OCID in the body. Unlike data-volume attach, AttachBootVolumeDetails is
// a concrete struct (boot-volume attachments are not polymorphic), so there is no
// attachment-type to choose.
package oracle_blockvolume_boot_volume_attachment_attach

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
	Name         = "OCI Block Volumes: Attach Boot Volume"
	Description  = "Attach an Oracle Cloud boot volume to a compute instance. Optionally set a display name and the in-transit encryption type."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the instance & boot volume pickers)"},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID", Placeholder: "ocid1.instance.oc1..aaaa… (the compute instance to attach to)", Required: true},
	{Name: "boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume OCID", Placeholder: "ocid1.bootvolume.oc1..aaaa… (the boot volume to attach)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the attachment (optional)"},
	{Name: "encryption_in_transit_type", Type: core.ConnectionTypeString, Label: "Encryption In Transit", Placeholder: "NONE or BM_ENCRYPTION_IN_TRANSIT (optional, defaults to NONE)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment", Type: core.ConnectionTypeObject, Label: "Boot Volume Attachment"},
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
	bootVolumeID, err := bv.RequiredString("boot_volume_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	details := ocicore.AttachBootVolumeDetails{InstanceId: &instanceID, BootVolumeId: &bootVolumeID}
	if v := strings.TrimSpace(bv.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(bv.OptionalString("encryption_in_transit_type", inputs)); v != "" {
		details.EncryptionInTransitType = ocicore.EncryptionInTransitTypeEnum(v)
	}
	resp, err := client.AttachBootVolume(bv.Context(), ocicore.AttachBootVolumeRequest{AttachBootVolumeDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	att := bv.SummariseBootVolumeAttachment(&resp.BootVolumeAttachment)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Attaching boot volume %s to instance %s (%s)", bootVolumeID, instanceID, att["lifecycle_state"]),
		"attachment":      att,
		"id":              att["id"],
		"lifecycle_state": att["lifecycle_state"],
		"success":         true,
	}, nil
}
