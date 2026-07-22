// Package oracle_filestorage_mount_target_upgrade_shape raises the throughput
// (shape) of an existing Oracle Cloud NFS mount target.
package oracle_filestorage_mount_target_upgrade_shape

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Upgrade Mount Target Shape"
	Description  = "Increase the throughput (shape) of an Oracle Cloud NFS mount target by setting a new requested throughput in Gbps."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the mount-target picker)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the mount-target picker)"},
	{Name: "mount_target_ocid", Type: core.ConnectionTypeString, Label: "Mount Target OCID", Placeholder: "ocid1.mounttarget.oc1..aaaa…", Required: true},
	{Name: "requested_throughput", Type: core.ConnectionTypeInteger, Label: "Requested Throughput (Gbps)", Placeholder: "New (higher) throughput in Gbps — e.g. 1, 3, 10, 20, 40 (see Mount Target Performance)", Required: true},
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

	resp, err := client.UpgradeShapeMountTarget(fss.Context(), filestorage.UpgradeShapeMountTargetRequest{
		MountTargetId: &id,
		UpgradeShapeMountTargetDetails: filestorage.UpgradeShapeMountTargetDetails{
			RequestedThroughput: &gbps,
		},
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	mt := fss.SummariseMountTarget(&resp.MountTarget)
	mt["observed_throughput"] = fss.Int64OrNil(resp.MountTarget.ObservedThroughput)
	mt["requested_throughput"] = fss.Int64OrNil(resp.MountTarget.RequestedThroughput)
	mt["reserved_storage_capacity"] = fss.Int64OrNil(resp.MountTarget.ReservedStorageCapacity)

	return fss.Result(
		fmt.Sprintf("Requested a throughput upgrade to %d Gbps for mount target %q (currently %s) — poll Get Mount Target until ACTIVE.", gbps, mt["display_name"], mt["lifecycle_state"]),
		map[string]interface{}{
			"mount_target": mt, "id": mt["id"], "lifecycle_state": mt["lifecycle_state"],
		}), nil
}
