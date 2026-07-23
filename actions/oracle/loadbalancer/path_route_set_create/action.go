// Package oracle_loadbalancer_path_route_set_create adds a path route set — a named
// collection of URL-path rules that route matching requests to specific backend sets —
// to a load balancer. Asynchronous — returns a work-request id.
package oracle_loadbalancer_path_route_set_create

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Create Path Route Set"
	Description  = "Add a path route set — a named collection of URL-path rules that route matching requests to specific backend sets — to an Oracle Cloud load balancer. Asynchronous."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "path_route_set_name", Type: core.ConnectionTypeString, Label: "Path Route Set Name", Placeholder: "A name unique within the load balancer, e.g. path-routes", Required: true},
	{Name: "path_routes_json", Type: core.ConnectionTypeString, Label: "Path Routes (JSON)", Placeholder: `[{"path":"/admin","backendSetName":"admin-servers","pathMatchType":{"matchType":"PREFIX_MATCH"}}] — matchType is EXACT_MATCH / FORCE_LONGEST_PREFIX_MATCH / PREFIX_MATCH / SUFFIX_MATCH`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "path_route_set_name", Type: core.ConnectionTypeString, Label: "Path Route Set Name"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	name, err := lbn.RequiredString("path_route_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	rawRoutes, err := lbn.RequiredString("path_routes_json", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// The path routes are a polymorphic body — decode the operator's JSON array
	// straight into the SDK slice type.
	var pathRoutes []lb.PathRoute
	if err := json.Unmarshal([]byte(rawRoutes), &pathRoutes); err != nil {
		return lbn.ErrorResult(fmt.Sprintf(`path routes must be a JSON array of {path, backendSetName, pathMatchType:{matchType}} objects, e.g. [{"path":"/admin","backendSetName":"admin-servers","pathMatchType":{"matchType":"PREFIX_MATCH"}}]: %s`, err.Error())), nil
	}
	if len(pathRoutes) == 0 {
		return lbn.ErrorResult("at least one path route is required"), nil
	}
	resp, err := client.CreatePathRouteSet(lbn.Context(), lb.CreatePathRouteSetRequest{
		LoadBalancerId: &lbID,
		CreatePathRouteSetDetails: lb.CreatePathRouteSetDetails{
			Name:       &name,
			PathRoutes: pathRoutes,
		},
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Creating path route set %q on load balancer %s — poll work request %s", name, lbID, lbn.Str(resp.OpcWorkRequestId)),
		"path_route_set_name": name,
		"work_request_id":     lbn.Str(resp.OpcWorkRequestId),
		"success":             true,
	}, nil
}
