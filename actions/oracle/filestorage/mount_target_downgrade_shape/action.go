// Package oracle_filestorage_mount_target_downgrade_shape schedules a throughput
// downgrade for an Oracle Cloud NFS mount target.
package oracle_filestorage_mount_target_downgrade_shape

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Downgrade Mount Target Shape"
	Description  = "Schedule a throughput downgrade for an Oracle Cloud NFS mount target, setting a new lower requested throughput in Gbps."
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
	{Name: "requested_throughput", Type: core.ConnectionTypeInteger, Label: "Requested Throughput (Gbps)", Placeholder: "New (lower) throughput in Gbps — e.g. 1", Required: true},
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

	throughput, ok, err := fss.OptionalInt("requested_throughput", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	if !ok {
		return fss.ErrorResult("requested throughput (Gbps) is required"), nil
	}
	gbps := int64(throughput)

	resp, err := client.ScheduleDowngradeShapeMountTarget(fss.Context(), filestorage.ScheduleDowngradeShapeMountTargetRequest{
		MountTargetId: &id,
		ScheduleDowngradeShapeMountTargetDetails: filestorage.ScheduleDowngradeShapeMountTargetDetails{
			RequestedThroughput: &gbps,
		},
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	mt := fss.SummariseMountTarget(&resp.MountTarget)
	return fss.Result(
		fmt.Sprintf("Scheduled a throughput downgrade to %d Gbps for mount target %q (currently %s) — poll Get Mount Target until ACTIVE.", gbps, mt["display_name"], mt["lifecycle_state"]),
		map[string]interface{}{
			"mount_target": mt, "id": mt["id"], "lifecycle_state": mt["lifecycle_state"],
		}), nil
}
