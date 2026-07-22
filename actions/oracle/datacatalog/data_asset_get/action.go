// Package oracle_datacatalog_data_asset_get fetches a single Data Catalog data asset by its key,
// returning its display name, description, type, external key and lifecycle state.
package oracle_datacatalog_data_asset_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Get Data Asset"
	Description  = "Fetch a single Data Catalog data asset by its key — its display name, type, external key and lifecycle state."
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
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa…", Required: true},
	{Name: "data_asset_key", Type: core.ConnectionTypeString, Label: "Data Asset Key", Placeholder: "The data asset's unique key (not an OCID)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "data_asset", Type: core.ConnectionTypeObject, Label: "Data Asset"},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Data Asset Key"},
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

	resp, err := client.GetDataAsset(dc.Context(), datacatalog.GetDataAssetRequest{
		CatalogId:    &catalogID,
		DataAssetKey: &dataAssetKey,
	})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}
	dataAsset := dc.SummariseDataAsset(&resp.DataAsset)
	return dc.Result(fmt.Sprintf("Data asset %q (%s)", dataAsset["display_name"], dataAsset["lifecycle_state"]), map[string]interface{}{
		"data_asset": dataAsset, "key": dataAsset["key"],
	}), nil
}
