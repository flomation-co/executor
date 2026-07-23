// Package oracle_filestorage_export_set_update updates an NFS export set's display
// name and its advanced FSSTAT reporting limits.
package oracle_filestorage_export_set_update

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
	Name         = "OCI File Storage: Update Export Set"
	Description  = "Rename an Oracle Cloud NFS export set and/or adjust its advanced FSSTAT byte and file reporting limits. Only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+share-from-square"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the export-set picker)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the export-set picker)"},
	{Name: "export_set_ocid", Type: core.ConnectionTypeString, Label: "Export Set OCID", Placeholder: "ocid1.exportset.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name for the export set (optional)"},
	{Name: "max_fs_stat_bytes", Type: core.ConnectionTypeString, Label: "Max FSSTAT Bytes", Placeholder: "Advanced: tbytes reported by NFS FSSTAT (optional)"},
	{Name: "max_fs_stat_files", Type: core.ConnectionTypeString, Label: "Max FSSTAT Files", Placeholder: "Advanced: tfiles reported by NFS FSSTAT (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "export_set", Type: core.ConnectionTypeObject, Label: "Export Set"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Export Set OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "export_set_ocid")
	if errResult != nil {
		return errResult, nil
	}

	var details filestorage.UpdateExportSetDetails
	if name := strings.TrimSpace(fss.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
	}
	if n, ok, err := fss.OptionalInt("max_fs_stat_bytes", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if ok {
		v := int64(n)
		details.MaxFsStatBytes = &v
	}
	if n, ok, err := fss.OptionalInt("max_fs_stat_files", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if ok {
		v := int64(n)
		details.MaxFsStatFiles = &v
	}

	resp, err := client.UpdateExportSet(fss.Context(), filestorage.UpdateExportSetRequest{
		ExportSetId:            &id,
		UpdateExportSetDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	set := fss.SummariseExportSet(&resp.ExportSet)
	return fss.Result(fmt.Sprintf("Export set %q is %s", set["display_name"], set["lifecycle_state"]), map[string]interface{}{
		"export_set":      set,
		"id":              set["id"],
		"lifecycle_state": set["lifecycle_state"],
	}), nil
}
