// Package oracle_datacatalog_catalog_list lists the Data Catalog instances in a compartment,
// optionally filtered by exact display name or lifecycle state, walking pagination up to a safe cap.
package oracle_datacatalog_catalog_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: List Catalogs"
	Description  = "List the Oracle Cloud Data Catalog instances in a compartment, optionally filtered by exact display name and lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only catalogs with this exact name, case-insensitive (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only catalogs in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Inactive", Value: "INACTIVE"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Moving", Value: "MOVING"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page, 1–100 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "catalogs", Type: core.ConnectionTypeObject, Label: "Catalogs"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	req := datacatalog.ListCatalogsRequest{CompartmentId: &compartment}
	if dn := dc.OptionalString("display_name", inputs); dn != "" {
		req.DisplayName = &dn
	}
	if state := strings.TrimSpace(dc.OptionalString("lifecycle_state", inputs)); state != "" {
		req.LifecycleState = datacatalog.ListCatalogsLifecycleStateEnum(strings.ToUpper(state))
	}
	if n, ok, err := dc.OptionalInt("limit", inputs); err != nil {
		return dc.ErrorResult(err.Error()), nil
	} else if ok {
		if n < 1 || n > 100 {
			return dc.ErrorResult("page size must be a whole number between 1 and 100"), nil
		}
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= dc.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListCatalogs(dc.Context(), req)
		if err != nil {
			return dc.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, dc.SummariseCatalogSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return dc.Result(fmt.Sprintf("Found %d catalog(s)", len(out)), map[string]interface{}{
		"catalogs": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
