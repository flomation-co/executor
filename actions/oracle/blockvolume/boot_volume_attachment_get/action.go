// Package oracle_blockvolume_boot_volume_attachment_get reads one boot-volume
// attachment by OCID. Boot-volume attachments live on the Compute client and, unlike
// data-volume attachments, are a concrete (non-polymorphic) struct.
package oracle_blockvolume_boot_volume_attachment_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Get Boot Volume Attachment"
	Description  = "Fetch a single Oracle Cloud boot-volume attachment by OCID — the instance and boot volume it links, its availability domain and lifecycle state."
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
	{Name: "boot_volume_attachment_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume Attachment OCID", Placeholder: "ocid1.bootvolumeattachment.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "boot_volume_attachment", Type: core.ConnectionTypeObject, Label: "Boot Volume Attachment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.ComputeResourceClient(inputs, "boot_volume_attachment_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetBootVolumeAttachment(bv.Context(), ocicore.GetBootVolumeAttachmentRequest{BootVolumeAttachmentId: &id})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	att := bv.SummariseBootVolumeAttachment(&resp.BootVolumeAttachment)
	return map[string]interface{}{
		"tool_result":            fmt.Sprintf("Boot volume attachment %q is %s", att["display_name"], att["lifecycle_state"]),
		"boot_volume_attachment": att,
		"id":                     att["id"],
		"lifecycle_state":        att["lifecycle_state"],
		"success":                true,
	}, nil
}
