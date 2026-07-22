// Package oracle_datacatalog_connection_list lists the connections defined on a data asset within a
// Data Catalog, optionally filtered by exact display name. Walks pagination up to a safe cap.
package oracle_datacatalog_connection_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: List Connections"
	Description  = "List the connections defined on a data asset in a Data Catalog. Optionally filter by exact display name. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+book"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa…", Required: true},
	{Name: "data_asset_key", Type: core.ConnectionTypeString, Label: "Data Asset Key", Placeholder: "The immutable key of the parent data asset", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only connections with this exact name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "connections", Type: core.ConnectionTypeObject, Label: "Connections"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dc.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	catalogID, err := dc.RequiredString("catalog_ocid", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	dataAssetKey, err := dc.RequiredString("data_asset_key", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	req := datacatalog.ListConnectionsRequest{CatalogId: &catalogID, DataAssetKey: &dataAssetKey}
	if name := dc.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= dc.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListConnections(dc.Context(), req)
		if err != nil {
			return dc.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, dc.SummariseConnectionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return dc.Result(fmt.Sprintf("Found %d connection(s)", len(out)), map[string]interface{}{
		"connections": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
