// Package oracle_networkloadbalancer_network_load_balancer_list_healths lists the
// overall health summary for every network load balancer in a compartment.
package oracle_networkloadbalancer_network_load_balancer_list_healths

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: List Network Load Balancer Healths"
	Description  = "List the overall health status (OK/WARNING/CRITICAL/UNKNOWN) for every Oracle Cloud network load balancer in a compartment. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health_summaries", Type: core.ConnectionTypeObject, Label: "Health Summaries"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := nlbn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	req := nlb.ListNetworkLoadBalancerHealthsRequest{CompartmentId: &compartment}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= nlbn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListNetworkLoadBalancerHealths(nlbn.Context(), req)
		if err != nil {
			return nlbn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, map[string]interface{}{
				"network_load_balancer_id": nlbn.Str(resp.Items[i].NetworkLoadBalancerId),
				"status":                   string(resp.Items[i].Status),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Found %d network load balancer health summary(ies)", len(out)),
		"health_summaries": out,
		"count":            fmt.Sprintf("%d", len(out)),
		"truncated":        truncated,
		"success":          true,
	}, nil
}
