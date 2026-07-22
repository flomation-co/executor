// Package oracle_loadbalancer_policy_list lists the backend-set load-balancing
// policies available in a compartment (ROUND_ROBIN, LEAST_CONNECTIONS, IP_HASH) —
// the valid values for a backend set's policy on Create/Update Backend Set.
package oracle_loadbalancer_policy_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: List Policies"
	Description  = "List the backend-set load-balancing policies available in a compartment (ROUND_ROBIN, LEAST_CONNECTIONS, IP_HASH) — the valid policy values for a backend set."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
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
	{Name: "policies", Type: core.ConnectionTypeObject, Label: "Policies"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lbn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	req := lb.ListPoliciesRequest{CompartmentId: &compartment}
	var names []string
	for page := 0; page < lbn.ListMaxPages; page++ {
		resp, err := client.ListPolicies(lbn.Context(), req)
		if err != nil {
			return lbn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			names = append(names, lbn.Str(resp.Items[i].Name))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d polic(ies)", len(names)),
		"policies":    names,
		"count":       fmt.Sprintf("%d", len(names)),
		"success":     true,
	}, nil
}
