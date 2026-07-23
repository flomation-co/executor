// Package oracle_filestorage_mount_target_update updates a mount target's mutable
// fields — its display name and its Network Security Group membership. The NSG list is
// REPLACE-semantics: supply nsg_ocids to replace the whole set, leave it blank to keep
// the current NSGs untouched. FSS is synchronous; the mount target is returned live.
package oracle_filestorage_mount_target_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Update Mount Target"
	Description  = "Update an Oracle Cloud mount target's display name and/or its Network Security Group membership. The NSG list is replace-all: supply NSG OCIDs to replace the whole set, leave it blank to keep the current NSGs unchanged."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
	Date         = "22/07/2026"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the mount-target picker)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the mount-target picker)"},
	{Name: "mount_target_ocid", Type: core.ConnectionTypeString, Label: "Mount Target OCID", Placeholder: "ocid1.mounttarget.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name (leave blank to keep current)"},
	{Name: "nsg_ocids", Type: core.ConnectionTypeString, Label: "NSG OCIDs (comma-separated)", Placeholder: "ocid1.networksecuritygroup…,… — replaces the whole set (leave blank to keep current)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mount_target", Type: core.ConnectionTypeObject, Label: "Mount Target"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Mount Target OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "mount_target_ocid")
	if errResult != nil {
		return errResult, nil
	}
	details := filestorage.UpdateMountTargetDetails{}
	if v := strings.TrimSpace(fss.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	// NSGs are REPLACE-semantics: overlay only when the operator supplies a list,
	// otherwise omit so the current membership is left unchanged.
	if nsgs := fss.InputStrings("nsg_ocids", inputs); len(nsgs) > 0 {
		details.NsgIds = nsgs
	}
	resp, err := client.UpdateMountTarget(fss.Context(), filestorage.UpdateMountTargetRequest{
		MountTargetId:            &id,
		UpdateMountTargetDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	mt := fss.SummariseMountTarget(&resp.MountTarget)
	return fss.Result(fmt.Sprintf("Updated mount target %q (%s)", mt["display_name"], mt["lifecycle_state"]), map[string]interface{}{
		"mount_target": mt, "id": mt["id"], "lifecycle_state": mt["lifecycle_state"],
	}), nil
}
