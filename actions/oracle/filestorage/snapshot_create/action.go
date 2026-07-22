// Package oracle_filestorage_snapshot_create takes a point-in-time snapshot of a file
// system. Snapshots are read-only and space-efficient; clone from one via Create File
// System's source snapshot.
package oracle_filestorage_snapshot_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Create Snapshot"
	Description  = "Take a point-in-time snapshot of an Oracle Cloud file system — read-only and space-efficient. Clone a new file system from it with Create File System's source snapshot. Poll Get Snapshot until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+clock-rotate-left"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the file-system picker)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the file-system picker)"},
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa… to snapshot", Required: true},
	{Name: "snapshot_name", Type: core.ConnectionTypeString, Label: "Snapshot Name", Placeholder: "A name for the snapshot (appears under .snapshot/)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot", Type: core.ConnectionTypeObject, Label: "Snapshot"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Snapshot OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	fileSystem, err := fss.RequiredString("file_system_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	name, err := fss.RequiredString("snapshot_name", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	details := filestorage.CreateSnapshotDetails{FileSystemId: &fileSystem, Name: &name}
	resp, err := client.CreateSnapshot(fss.Context(), filestorage.CreateSnapshotRequest{CreateSnapshotDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	snap := fss.SummariseSnapshot(&resp.Snapshot)
	return fss.Result(fmt.Sprintf("Creating snapshot %q (%s) — poll Get Snapshot until ACTIVE", name, snap["lifecycle_state"]), map[string]interface{}{
		"snapshot": snap, "id": snap["id"], "lifecycle_state": snap["lifecycle_state"],
	}), nil
}
