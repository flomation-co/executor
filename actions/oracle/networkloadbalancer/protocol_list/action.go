// Package oracle_networkloadbalancer_protocol_list lists the listener protocols a
// network load balancer supports (ANY, TCP, UDP, TCP_AND_UDP, L3IP) — the valid
// values for the protocol on Create Listener.
package oracle_networkloadbalancer_protocol_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: List Protocols"
	Description  = "List the listener protocols a network load balancer supports (ANY, TCP, UDP, TCP_AND_UDP, L3IP) — the valid protocol values for Create Listener."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "protocols", Type: core.ConnectionTypeObject, Label: "Protocols"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := nlbn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	req := nlb.ListNetworkLoadBalancersProtocolsRequest{}
	var names []string
	for page := 0; page < nlbn.ListMaxPages; page++ {
		resp, err := client.ListNetworkLoadBalancersProtocols(nlbn.Context(), req)
		if err != nil {
			return nlbn.ErrorResult(auth.OCIError(err)), nil
		}
		for _, p := range resp.Items {
			names = append(names, string(p))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d protocol(s)", len(names)),
		"protocols":   names,
		"count":       fmt.Sprintf("%d", len(names)),
		"success":     true,
	}, nil
}
