// Package oracle_filestorage_export_list lists NFS exports — the links between file
// systems and export sets at a mount path. Exports are NOT availability-domain-scoped, so
// the compartment and the export-set / file-system filters are all optional (the OCI API
// takes any combination).
package oracle_filestorage_export_list

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
	Name         = "OCI File Storage: List Exports"
	Description  = "List the Oracle Cloud NFS exports (the links between file systems and export sets), optionally scoped by compartment and filtered by export set or file system. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (optional — scopes the export-set / file-system pickers)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the file-system / export-set picker; not otherwise used)"},
	{Name: "export_set_ocid", Type: core.ConnectionTypeString, Label: "Export Set OCID Filter", Placeholder: "ocid1.exportset.oc1..aaaa… — only exports in this export set (optional)"},
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID Filter", Placeholder: "ocid1.filesystem.oc1..aaaa… — only exports of this file system (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "exports", Type: core.ConnectionTypeObject, Label: "Exports"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	req := filestorage.ListExportsRequest{}
	if v := strings.TrimSpace(fss.OptionalString("compartment_ocid", inputs)); v != "" {
		req.CompartmentId = &v
	}
	if v := strings.TrimSpace(fss.OptionalString("export_set_ocid", inputs)); v != "" {
		req.ExportSetId = &v
	}
	if v := strings.TrimSpace(fss.OptionalString("file_system_ocid", inputs)); v != "" {
		req.FileSystemId = &v
	}
	if req.CompartmentId == nil && req.ExportSetId == nil && req.FileSystemId == nil {
		return fss.ErrorResult("at least one of compartment_ocid, export_set_ocid or file_system_ocid is required to list exports"), nil
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fss.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListExports(fss.Context(), req)
		if err != nil {
			return fss.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, fss.SummariseExportSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return fss.Result(fmt.Sprintf("Found %d export(s)", len(out)), map[string]interface{}{
		"exports": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
