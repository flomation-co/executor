// Package oracle_loadbalancer_path_route_set_list lists the path-route sets of a
// load balancer.
package oracle_loadbalancer_path_route_set_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: List Path Route Sets"
	Description  = "List the path-route sets of an Oracle Cloud load balancer — each with its ordered path routes."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "path_route_sets", Type: core.ConnectionTypeObject, Label: "Path Route Sets"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	// ListPathRouteSets returns a bare []PathRouteSet with no next-page token — one call.
	resp, err := client.ListPathRouteSets(lbn.Context(), lb.ListPathRouteSetsRequest{LoadBalancerId: &lbID})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	var out []map[string]interface{}
	for i := range resp.Items {
		out = append(out, lbn.SummarisePathRouteSet(&resp.Items[i]))
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Found %d path route set(s)", len(out)),
		"path_route_sets": out,
		"count":           fmt.Sprintf("%d", len(out)),
		"success":         true,
	}, nil
}
