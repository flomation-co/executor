// Package oracle_nosql_row_get fetches a single row from a NoSQL Database table by its primary
// key, returning the row's column values.
package oracle_nosql_row_get

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: Get Row"
	Description  = "Fetch a single row from a NoSQL Database table by its primary key — the row's column values."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+table"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — needed when identifying the table by name"},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table OCID or Name", Placeholder: "ocid1.nosqltable.oc1..aaaa… or a plain table name", Required: true},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Primary Key", Placeholder: "Comma-separated \"column:value\" pairs in PK order, e.g. id:42,region:emea", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "row", Type: core.ConnectionTypeObject, Label: "Row"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ns.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	tableRef, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	keyRaw, err := ns.RequiredString("key", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	key := make([]string, 0)
	for _, p := range strings.Split(keyRaw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.Contains(p, ":") {
			return ns.ErrorResult(fmt.Sprintf("key component %q must be in \"column:value\" form", p)), nil
		}
		key = append(key, p)
	}
	if len(key) == 0 {
		return ns.ErrorResult("key is required — provide the primary key as comma-separated \"column:value\" pairs"), nil
	}

	req := nosql.GetRowRequest{TableNameOrId: &tableRef, Key: key}
	if auth.CompartmentOCID != "" {
		cid := auth.CompartmentOCID
		req.CompartmentId = &cid
	}

	resp, err := client.GetRow(ns.Context(), req)
	if err != nil {
		return ns.ErrorResult(auth.OCIError(err)), nil
	}
	if resp.Value == nil {
		return ns.Result(fmt.Sprintf("No row found in table %q for the given key", tableRef), map[string]interface{}{
			"row": nil,
		}), nil
	}
	return ns.Result(fmt.Sprintf("Fetched row from table %q (%d column(s))", tableRef, len(resp.Value)), map[string]interface{}{
		"row": resp.Value,
	}), nil
}
