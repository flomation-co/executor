// Package oracle_datacatalog_catalog_update applies a partial update to a Data Catalog instance:
// only the display name you supply is changed, and a blank field is left unchanged. Asynchronous:
// the update returns a work-request id — poll Get Catalog until the instance is ACTIVE again.
package oracle_datacatalog_catalog_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Update Catalog"
	Description  = "Partially update a Data Catalog instance — change only the display name you supply; a blank field is left unchanged. Returns a work-request id; poll Get Catalog until ACTIVE."
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
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa… — the catalog to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
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

	// Partial update: only carry the display name when the operator supplied one.
	details := datacatalog.UpdateCatalogDetails{}
	if name := dc.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.UpdateCatalog(dc.Context(), datacatalog.UpdateCatalogRequest{
		CatalogId:            &catalogID,
		UpdateCatalogDetails: details,
	})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}

	// UpdateCatalogResponse has no OpcWorkRequestId field — the async work-request id arrives on the
	// raw HTTP header instead.
	workRequestID := ""
	if resp.RawResponse != nil {
		workRequestID = resp.RawResponse.Header.Get("opc-work-request-id")
	}

	return dc.Result(fmt.Sprintf("Updating Data Catalog %s — poll Get Catalog until ACTIVE", dc.Str(resp.Catalog.Id)), map[string]interface{}{
		"id":              dc.Str(resp.Catalog.Id),
		"work_request_id": workRequestID,
	}), nil
}
