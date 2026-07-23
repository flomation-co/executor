// Package oracle_filestorage_export_update replaces an export's NFS export options —
// the per-client access rules governing who may mount the file system through this export.
package oracle_filestorage_export_update

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
	Name         = "OCI File Storage: Update Export"
	Description  = "Replace an export's NFS export options — the per-client access rules (source CIDR, read-write/read-only, identity squash) that govern who may mount the file system through this export. The supplied list fully replaces the current options."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the export picker)"},
	{Name: "export_ocid", Type: core.ConnectionTypeString, Label: "Export OCID", Placeholder: "ocid1.export.oc1..aaaa…", Required: true},
	{Name: "export_options_json", Type: core.ConnectionTypeText, Label: "Export Options (JSON array)", Placeholder: `[{"source":"10.0.0.0/24","access":"READ_WRITE","identitySquash":"NONE"}] — fully replaces the current options; [] makes the export invisible to all clients`, Required: true},
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
	auth, client, id, errResult := fss.ResourceClient(inputs, "export_ocid")
	if errResult != nil {
		return errResult, nil
	}
	raw, err := fss.RequiredString("export_options_json", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	var opts []filestorage.ClientOptions
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &opts); err != nil {
		return fss.ErrorResult(fmt.Sprintf(`export options must be a JSON array of client options, e.g. [{"source":"10.0.0.0/24","access":"READ_WRITE"}] (use [] to make the export invisible): %s`, err.Error())), nil
	}
	details := filestorage.UpdateExportDetails{ExportOptions: opts}
	resp, err := client.UpdateExport(fss.Context(), filestorage.UpdateExportRequest{ExportId: &id, UpdateExportDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	export := fss.SummariseExport(&resp.Export)
	return fss.Result(fmt.Sprintf("Updated export options for %q (%d rule(s), %s)", export["path"], len(opts), export["lifecycle_state"]), map[string]interface{}{
		"export": export, "id": export["id"], "lifecycle_state": export["lifecycle_state"],
	}), nil
}
