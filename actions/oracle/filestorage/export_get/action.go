// Package oracle_filestorage_export_get reads one NFS export by OCID.
package oracle_filestorage_export_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Get Export"
	Description  = "Fetch a single Oracle Cloud NFS export by OCID — the file system it links, its export set, the mount path and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+link"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the export picker)"},
	{Name: "export_ocid", Type: core.ConnectionTypeString, Label: "Export OCID", Placeholder: "ocid1.export.oc1..aaaa…", Required: true},
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
	resp, err := client.GetExport(fss.Context(), filestorage.GetExportRequest{ExportId: &id})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	ex := fss.SummariseExport(&resp.Export)
	return fss.Result(fmt.Sprintf("Export %q is %s", ex["path"], ex["lifecycle_state"]), map[string]interface{}{
		"export": ex, "id": ex["id"], "lifecycle_state": ex["lifecycle_state"],
	}), nil
}
