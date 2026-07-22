// Package oracle_mysql_configuration_list lists the MySQL configurations in a compartment,
// optionally filtered by exact display name, type (DEFAULT/CUSTOM) or shape name. Walks
// pagination up to a safe cap.
package oracle_mysql_configuration_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: List Configurations"
	Description  = "List the MySQL configurations in a compartment. Optionally filter by exact display name, type (DEFAULT or CUSTOM) or shape name. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+database"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only configurations with this exact name (optional)"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Filter by configuration type (optional)", Options: []core.ConnectionOption{
		{Name: "Default", Value: "DEFAULT"}, {Name: "Custom", Value: "CUSTOM"},
	}},
	{Name: "shape_name", Type: core.ConnectionTypeString, Label: "Shape Name Filter", Placeholder: "Only configurations for this shape, e.g. MySQL.VM.Standard.E3.1.8GB (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "configurations", Type: core.ConnectionTypeObject, Label: "Configurations"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.ConfigClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	req := mysql.ListConfigurationsRequest{CompartmentId: &compartment}
	if name := my.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if t := my.OptionalString("type", inputs); t != "" {
		req.Type = []mysql.ListConfigurationsTypeEnum{mysql.ListConfigurationsTypeEnum(t)}
	}
	if shape := my.OptionalString("shape_name", inputs); shape != "" {
		req.ShapeName = &shape
	}
	if limit, ok, err := my.OptionalInt("limit", inputs); err != nil {
		return my.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= my.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListConfigurations(my.Context(), req)
		if err != nil {
			return my.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, my.SummariseConfigurationSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return my.Result(fmt.Sprintf("Found %d configuration(s)", len(out)), map[string]interface{}{
		"configurations": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
