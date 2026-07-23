// Package oracle_blockvolume_volume_attachment_update updates the mutable fields of
// a volume attachment. The Update call lives on the Compute client, keyed by the
// attachment OCID (path). The only field the OCI API exposes for update is the iSCSI
// login state (LOGIN_SUCCEEDED / LOGOUT_SUCCEEDED etc.), used to drive multipath
// iSCSI sessions in or out of the logged-in state — so that is the single operator
// knob here; the returned (polymorphic) attachment model is summarised on success.
package oracle_blockvolume_volume_attachment_update

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
	Name         = "OCI Block Volumes: Update Volume Attachment"
	Description  = "Update the mutable fields of an Oracle Cloud volume attachment — set the iSCSI login state to drive multipath iSCSI sessions in or out of the logged-in state."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the attachment picker)"},
	{Name: "volume_attachment_ocid", Type: core.ConnectionTypeString, Label: "Volume Attachment OCID", Placeholder: "ocid1.volumeattachment.oc1..aaaa…", Required: true},
	{Name: "iscsi_login_state", Type: core.ConnectionTypeString, Label: "iSCSI Login State", Placeholder: "LOGIN_SUCCEEDED or LOGOUT_SUCCEEDED — drive multipath iSCSI sessions in/out (optional)"},
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
	auth, client, id, errResult := bv.ComputeResourceClient(inputs, "volume_attachment_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := ocicore.UpdateVolumeAttachmentDetails{}
	if v := strings.TrimSpace(bv.OptionalString("iscsi_login_state", inputs)); v != "" {
		state, ok := ocicore.GetMappingUpdateVolumeAttachmentDetailsIscsiLoginStateEnum(v)
		if !ok {
			return bv.ErrorResult(fmt.Sprintf("iSCSI login state %q is not valid — expected one of: %s", v, strings.Join(ocicore.GetUpdateVolumeAttachmentDetailsIscsiLoginStateEnumStringValues(), ", "))), nil
		}
		details.IscsiLoginState = state
	}

	resp, err := client.UpdateVolumeAttachment(bv.Context(), ocicore.UpdateVolumeAttachmentRequest{VolumeAttachmentId: &id, UpdateVolumeAttachmentDetails: details})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	att := bv.SummariseVolumeAttachment(resp.VolumeAttachment)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated volume attachment %s (%s)", id, att["lifecycle_state"]),
		"attachment":      att,
		"id":              att["id"],
		"lifecycle_state": att["lifecycle_state"],
		"success":         true,
	}, nil
}
