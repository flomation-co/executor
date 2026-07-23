// Package oracle_filestorage_file_system_create creates an NFS file system in an
// availability domain. Provisioning is synchronous-ish — the call returns the file
// system with its OCID in a CREATING state; poll Get File System until it is ACTIVE.
package oracle_filestorage_file_system_create

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
	Name         = "OCI File Storage: Create File System"
	Description  = "Create an Oracle Cloud NFS file system in an availability domain. Mount it by wiring an Export to a mount target. Returns the OCID immediately in a CREATING state; poll Get File System until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+folder-tree"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the file system", Required: true},
	{Name: "kms_key_ocid", Type: core.ConnectionTypeString, Label: "KMS Key OCID", Placeholder: "Customer-managed encryption key (optional; defaults to Oracle-managed)"},
	{Name: "source_snapshot_ocid", Type: core.ConnectionTypeString, Label: "Source Snapshot OCID", Placeholder: "Clone from this snapshot (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file_system", Type: core.ConnectionTypeObject, Label: "File System"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "File System OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	ad, err := fss.RequiredAvailabilityDomain(inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	displayName, err := fss.RequiredString("display_name", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	details := filestorage.CreateFileSystemDetails{CompartmentId: &compartment, AvailabilityDomain: &ad, DisplayName: &displayName}
	if v := strings.TrimSpace(fss.OptionalString("kms_key_ocid", inputs)); v != "" {
		details.KmsKeyId = &v
	}
	if v := strings.TrimSpace(fss.OptionalString("source_snapshot_ocid", inputs)); v != "" {
		details.SourceSnapshotId = &v
	}
	if tags, err := fss.FreeformTags("tags", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateFileSystem(fss.Context(), filestorage.CreateFileSystemRequest{CreateFileSystemDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	fs := fss.SummariseFileSystem(&resp.FileSystem)
	return fss.Result(fmt.Sprintf("Creating file system %q (%s) — poll Get File System until ACTIVE", displayName, fs["lifecycle_state"]), map[string]interface{}{
		"file_system": fs, "id": fs["id"], "lifecycle_state": fs["lifecycle_state"],
	}), nil
}
