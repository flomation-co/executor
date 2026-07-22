// Package oracle_blockvolume_volume_detach detaches a block volume from an instance,
// keyed by the volume-attachment OCID. Detach is asynchronous — OCI returns a work-
// request id and completes the detach in the background.
package oracle_blockvolume_volume_detach

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Detach Volume"
	Description  = "Detach an Oracle Cloud block volume from a compute instance, keyed by the volume-attachment OCID. Detach is asynchronous and returns a work-request id."
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
	{Name: "volume_attachment_ocid", Type: core.ConnectionTypeString, Label: "Volume Attachment OCID", Placeholder: "ocid1.volumeattachment.oc1..aaaa… (from Attach Volume / List Attachments)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.ComputeResourceClient(inputs, "volume_attachment_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.DetachVolume(bv.Context(), ocicore.DetachVolumeRequest{VolumeAttachmentId: &id})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Detach requested for volume attachment %s", id),
		"id":              id,
		"work_request_id": bv.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
