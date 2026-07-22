// Package oracle_datacatalog_glossary_get fetches a single Data Catalog business glossary by its
// key, returning the glossary's display name, description, owner, workflow status and lifecycle
// state.
package oracle_datacatalog_glossary_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Get Glossary"
	Description  = "Fetch a single Data Catalog business glossary by its key — its display name, description, owner, workflow status and lifecycle state."
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
	{Name: "glossary_key", Type: core.ConnectionTypeString, Label: "Glossary Key", Placeholder: "The immutable key of the glossary", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "glossary", Type: core.ConnectionTypeObject, Label: "Glossary"},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Glossary Key"},
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
	glossaryKey, err := dc.RequiredString("glossary_key", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetGlossary(dc.Context(), datacatalog.GetGlossaryRequest{
		CatalogId:   &catalogID,
		GlossaryKey: &glossaryKey,
	})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}
	glossary := dc.SummariseGlossary(&resp.Glossary)
	return dc.Result(fmt.Sprintf("Glossary %q (%s)", glossary["display_name"], glossary["lifecycle_state"]), map[string]interface{}{
		"glossary": glossary, "key": glossary["key"],
	}), nil
}
