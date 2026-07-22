// Package oracle_datacatalog_data_asset_update applies a partial update to a Data Catalog data
// asset: only the display name and/or description you supply are changed; blank fields are left
// unchanged. The data asset is a child resource identified by its string key and scoped by the
// parent catalog OCID.
package oracle_datacatalog_data_asset_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Update Data Asset"
	Description  = "Partially update a Data Catalog data asset — change only the display name or description you supply; blank fields are left unchanged."
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
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa… — the catalog the data asset belongs to", Required: true},
	{Name: "data_asset_key", Type: core.ConnectionTypeString, Label: "Data Asset Key", Placeholder: "The data asset's unique key (not an OCID)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
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
	assetKey, err := dc.RequiredString("data_asset_key", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied so blank inputs leave the
	// existing values unchanged.
	details := datacatalog.UpdateDataAssetDetails{}
	if v := dc.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := dc.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateDataAsset(dc.Context(), datacatalog.UpdateDataAssetRequest{
		CatalogId:              &catalogID,
		DataAssetKey:           &assetKey,
		UpdateDataAssetDetails: details,
	})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}
	asset := dc.SummariseDataAsset(&resp.DataAsset)
	return dc.Result(fmt.Sprintf("Updated data asset %q (%s)", asset["display_name"], asset["lifecycle_state"]), map[string]interface{}{
		"data_asset": asset, "key": asset["key"],
	}), nil
}
