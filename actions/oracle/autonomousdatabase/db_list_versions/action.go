// Package oracle_autonomousdatabase_db_list_versions lists the available
// Autonomous Database engine versions in an Oracle Cloud compartment, optionally
// filtered by workload type.
package oracle_autonomousdatabase_db_list_versions

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: List DB Versions"
	Description  = "List the available Autonomous Database engine versions in a compartment, optionally filtered by workload."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
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
	{Name: "db_workload", Type: core.ConnectionTypeString, Label: "Workload Filter", Placeholder: "Only versions for this workload type (optional)", Options: []core.ConnectionOption{
		{Name: "Transaction Processing (OLTP)", Value: "OLTP"},
		{Name: "Data Warehouse (DW)", Value: "DW"},
		{Name: "JSON Database (AJD)", Value: "AJD"},
		{Name: "APEX", Value: "APEX"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "versions", Type: core.ConnectionTypeObject, Label: "Versions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := adb.GetAuth(inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := adb.Context()

	req := database.ListAutonomousDbVersionsRequest{CompartmentId: &compartment}
	if v := strings.TrimSpace(adb.OptionalString("db_workload", inputs)); v != "" {
		req.DbWorkload = database.AutonomousDatabaseSummaryDbWorkloadEnum(v)
	}

	var versions []map[string]interface{}
	truncated := false
	for page := 0; page < adb.ListMaxPages; page++ {
		resp, err := client.ListAutonomousDbVersions(ctx, req)
		if err != nil {
			return adb.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			v := &resp.Items[i]
			versions = append(versions, map[string]interface{}{
				"version":              adb.Str(v.Version),
				"db_workload":          string(v.DbWorkload),
				"details":              adb.Str(v.Details),
				"is_dedicated":         v.IsDedicated != nil && *v.IsDedicated,
				"is_free_tier_enabled": v.IsFreeTierEnabled != nil && *v.IsFreeTierEnabled,
				"is_dev_tier_enabled":  v.IsDevTierEnabled != nil && *v.IsDevTierEnabled,
				"is_paid_enabled":      v.IsPaidEnabled != nil && *v.IsPaidEnabled,
				"is_default_for_free":  v.IsDefaultForFree != nil && *v.IsDefaultForFree,
				"is_default_for_paid":  v.IsDefaultForPaid != nil && *v.IsDefaultForPaid,
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == adb.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d Autonomous Database version(s) in the compartment", len(versions))
	if truncated {
		summary = fmt.Sprintf("Found at least %d Autonomous Database version(s) (list truncated at %d pages — more available)", len(versions), adb.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"versions":    versions,
		"count":       len(versions),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
