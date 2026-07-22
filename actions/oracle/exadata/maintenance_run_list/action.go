// Package oracle_exadata_maintenance_run_list lists the maintenance runs in a
// compartment, optionally scoped to a single target resource.
package oracle_exadata_maintenance_run_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: List Maintenance Runs"
	Description  = "List the maintenance runs in a compartment, optionally filtered to one target resource. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "target_resource_ocid", Type: core.ConnectionTypeString, Label: "Target Resource OCID", Placeholder: "Only maintenance runs for this resource (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "maintenance_runs", Type: core.ConnectionTypeObject, Label: "Maintenance runs"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	req := db.ListMaintenanceRunsRequest{CompartmentId: &compartment}
	if target := exa.OptionalString("target_resource_ocid", inputs); target != "" {
		req.TargetResourceId = &target
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= exa.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListMaintenanceRuns(exa.Context(), req)
		if err != nil {
			return exa.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, exa.SummariseMaintenanceRunSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return exa.Result(fmt.Sprintf("Found %d maintenance run(s)", len(out)), map[string]interface{}{
		"maintenance_runs": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
