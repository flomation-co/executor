// Package oracle_datacatalog_term_create creates a business term inside a glossary: a controlled
// piece of vocabulary that gives data assets shared business meaning.
package oracle_datacatalog_term_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Create Term"
	Description  = "Create a business term in a glossary — a controlled piece of vocabulary that gives your data assets shared business meaning."
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
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa… — the catalog the glossary belongs to", Required: true},
	{Name: "glossary_key", Type: core.ConnectionTypeString, Label: "Glossary Key", Placeholder: "The key of the glossary to create the term in", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the term", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "term", Type: core.ConnectionTypeObject, Label: "Term"},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Term Key"},
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
	name, err := dc.RequiredString("display_name", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}

	details := datacatalog.CreateTermDetails{DisplayName: &name}
	if d := dc.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}

	resp, err := client.CreateTerm(dc.Context(), datacatalog.CreateTermRequest{
		CatalogId:         &catalogID,
		GlossaryKey:       &glossaryKey,
		CreateTermDetails: details,
	})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}

	term := dc.SummariseTerm(&resp.Term)
	return dc.Result(fmt.Sprintf("Created term %q (%s)", term["display_name"], term["lifecycle_state"]), map[string]interface{}{
		"term": term, "key": term["key"],
	}), nil
}
