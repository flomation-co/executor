// Package oracle_filestorage_file_system_update renames / re-tags an NFS file system.
package oracle_filestorage_file_system_update

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
	Name         = "OCI File Storage: Update File System"
	Description  = "Update an Oracle Cloud NFS file system's display name and/or free-form tags. Only the fields you supply are changed; the rest are left as-is."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+folder-tree"
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
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep the current one)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Free-form Tags (JSON)", Placeholder: `e.g. {"env":"prod"} — replaces all free-form tags; leave blank to keep`},
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
	auth, client, id, errResult := fss.ResourceClient(inputs, "file_system_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := filestorage.UpdateFileSystemDetails{}
	changed := false

	if name := strings.TrimSpace(fss.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
		changed = true
	}

	tags, err := fss.FreeformTags("tags", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	if tags != nil {
		details.FreeformTags = tags
		changed = true
	}

	if !changed {
		return fss.ErrorResult("nothing to update — supply a display name and/or free-form tags"), nil
	}

	resp, err := client.UpdateFileSystem(fss.Context(), filestorage.UpdateFileSystemRequest{
		FileSystemId:            &id,
		UpdateFileSystemDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	fs := fss.SummariseFileSystem(&resp.FileSystem)
	return fss.Result(fmt.Sprintf("Updated file system %q (%s)", fs["display_name"], fs["lifecycle_state"]), map[string]interface{}{
		"file_system": fs, "id": fs["id"], "lifecycle_state": fs["lifecycle_state"],
	}), nil
}
