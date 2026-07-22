// Package oracle_datacatalog_catalog_create creates a Data Catalog — the OCID-identified regional
// resource that every data asset, glossary, term and entity is then scoped under. Asynchronous: the
// catalog comes back with a work-request id in a CREATING state; poll Get Catalog until it is ACTIVE
// before harvesting into it.
package oracle_datacatalog_catalog_create

import (
	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Create Catalog"
	Description  = "Create a Data Catalog instance in a compartment. Returns a work-request id — poll Get Catalog until it is ACTIVE before use."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the catalog (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dc.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}

	details := datacatalog.CreateCatalogDetails{CompartmentId: &compartment}
	if name := dc.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateCatalog(dc.Context(), datacatalog.CreateCatalogRequest{CreateCatalogDetails: details})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}

	return dc.Result("Creating Data Catalog — poll Get Catalog until ACTIVE", map[string]interface{}{
		"work_request_id": dc.Str(resp.OpcWorkRequestId),
	}), nil
}
