// Package oracle_apigateway_gateway_list lists the API gateways in a compartment, optionally
// filtered by display name or lifecycle state. Walks pagination up to a safe cap.
package oracle_apigateway_gateway_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: List Gateways"
	Description  = "List the API gateways in a compartment. Optionally filter by exact display name or lifecycle state, and cap the per-page size. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only gateways with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only gateways in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"}, {Name: "Active", Value: "ACTIVE"}, {Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"}, {Name: "Deleted", Value: "DELETED"}, {Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "gateways", Type: core.ConnectionTypeObject, Label: "Gateways"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.GatewayClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	req := apigateway.ListGatewaysRequest{CompartmentId: &compartment}
	if name := agw.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := agw.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = apigateway.GatewayLifecycleStateEnum(state)
	}
	if limit, ok, err := agw.OptionalInt("limit", inputs); err != nil {
		return agw.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= agw.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListGateways(agw.Context(), req)
		if err != nil {
			return agw.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, agw.SummariseGatewaySummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return agw.Result(fmt.Sprintf("Found %d gateway(s)", len(out)), map[string]interface{}{
		"gateways": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
