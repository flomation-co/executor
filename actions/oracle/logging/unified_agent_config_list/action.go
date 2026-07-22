// Package oracle_logging_unified_agent_config_list lists the unified monitoring agent configurations
// in a compartment, optionally filtered by exact display name. Walks pagination up to a safe cap.
package oracle_logging_unified_agent_config_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: List Unified Agent Configurations"
	Description  = "List the unified monitoring agent configurations in a compartment. Optionally filter by exact display name. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
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
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Records fetched per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "configs", Type: core.ConnectionTypeObject, Label: "Unified Agent Configurations"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lg.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	req := logging.ListUnifiedAgentConfigurationsRequest{CompartmentId: &compartment}
	if name := lg.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if limit, ok, err := lg.OptionalInt("limit", inputs); err != nil {
		return lg.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= lg.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListUnifiedAgentConfigurations(lg.Context(), req)
		if err != nil {
			return lg.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, lg.SummariseUnifiedAgentConfigurationSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return lg.Result(fmt.Sprintf("Found %d unified agent configuration(s)", len(out)), map[string]interface{}{
		"configs": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
