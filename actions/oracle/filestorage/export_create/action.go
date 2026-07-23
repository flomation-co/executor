// Package oracle_filestorage_export_create creates an export — the link that makes a
// file system reachable through a mount target's export set at a given NFS path.
package oracle_filestorage_export_create

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Create Export"
	Description  = "Create an export — the link that makes a file system reachable through a mount target's export set at an NFS path (e.g. /shared). This is what clients actually mount. Optionally restrict access with export options."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+link"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the pickers)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the file-system / export-set picker; not otherwise used)"},
	{Name: "export_set_ocid", Type: core.ConnectionTypeString, Label: "Export Set OCID", Placeholder: "ocid1.exportset.oc1..aaaa… (from the mount target)", Required: true},
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa…", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Export Path", Placeholder: "The NFS path clients mount, e.g. /shared", Required: true},
	{Name: "export_options_json", Type: core.ConnectionTypeText, Label: "Export Options (JSON array)", Placeholder: `[{"source":"10.0.0.0/24","access":"READ_WRITE","identitySquash":"NONE"}] — restrict access (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "export", Type: core.ConnectionTypeObject, Label: "Export"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Export OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	exportSet, err := fss.RequiredString("export_set_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	fileSystem, err := fss.RequiredString("file_system_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	path, err := fss.RequiredString("path", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	details := filestorage.CreateExportDetails{ExportSetId: &exportSet, FileSystemId: &fileSystem, Path: &path}
	if raw := strings.TrimSpace(fss.OptionalString("export_options_json", inputs)); raw != "" {
		var opts []filestorage.ClientOptions
		if err := json.Unmarshal([]byte(raw), &opts); err != nil {
			return fss.ErrorResult(fmt.Sprintf(`export options must be a JSON array of client options, e.g. [{"source":"10.0.0.0/24","access":"READ_WRITE"}]: %s`, err.Error())), nil
		}
		details.ExportOptions = opts
	}
	resp, err := client.CreateExport(fss.Context(), filestorage.CreateExportRequest{CreateExportDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	export := fss.SummariseExport(&resp.Export)
	return fss.Result(fmt.Sprintf("Created export at %q (%s)", path, export["lifecycle_state"]), map[string]interface{}{
		"export": export, "id": export["id"], "lifecycle_state": export["lifecycle_state"],
	}), nil
}
