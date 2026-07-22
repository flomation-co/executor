// Package oracle_networkloadbalancer_backend_list lists the backend servers of one
// backend set of a network load balancer.
package oracle_networkloadbalancer_backend_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: List Backends"
	Description  = "List the backend servers of one backend set of an Oracle Cloud network load balancer. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set name, e.g. app-servers", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backends", Type: core.ConnectionTypeObject, Label: "Backends"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// summariseBackendSummary mirrors nlbn.SummariseBackend for the list endpoint, which
// returns the distinct (but field-identical) BackendSummary type.
func summariseBackendSummary(b *nlb.BackendSummary) map[string]interface{} {
	m := map[string]interface{}{
		"name":       nlbn.Str(b.Name),
		"ip_address": nlbn.Str(b.IpAddress),
		"target_id":  nlbn.Str(b.TargetId),
		"is_drain":   b.IsDrain != nil && *b.IsDrain,
		"is_backup":  b.IsBackup != nil && *b.IsBackup,
		"is_offline": b.IsOffline != nil && *b.IsOffline,
	}
	if b.Port != nil {
		m["port"] = *b.Port
	}
	if b.Weight != nil {
		m["weight"] = *b.Weight
	}
	return m
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, nlbID, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	backendSetName, err := nlbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	req := nlb.ListBackendsRequest{NetworkLoadBalancerId: &nlbID, BackendSetName: &backendSetName}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= nlbn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListBackends(nlbn.Context(), req)
		if err != nil {
			return nlbn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseBackendSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d backend(s) in backend set %q", len(out), backendSetName),
		"backends":    out,
		"count":       fmt.Sprintf("%d", len(out)),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
