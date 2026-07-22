// Package oracle_datacatalog_catalog_delete deletes a Data Catalog instance by its OCID. The catalog
// and everything harvested into it (data assets, connections, glossaries, entities) go with it.
// Asynchronous: the delete returns a work-request id — track it to confirm the catalog is gone.
package oracle_datacatalog_catalog_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Delete Catalog"
	Description  = "Delete a Data Catalog instance by its OCID, removing it and every data asset, connection and glossary harvested into it. Returns a work-request id to track the asynchronous delete."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa… of the catalog to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Catalog OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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

	resp, err := client.DeleteCatalog(dc.Context(), datacatalog.DeleteCatalogRequest{CatalogId: &catalogID})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}
	return dc.Result(fmt.Sprintf("Deleting catalog %s — track the work request to confirm removal", catalogID), map[string]interface{}{
		"id":              catalogID,
		"work_request_id": dc.Str(resp.OpcWorkRequestId),
	}), nil
}
