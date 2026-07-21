// Package oracle_networking_service_list_all lists the Oracle services available
// to a service gateway in the configured region.
package oracle_networking_service_list_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: List Services"
	Description  = "List the Oracle services available to a service gateway in this region (their OCIDs go into Create Service Gateway)."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "services", Type: core.ConnectionTypeObject, Label: "Services"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := net.GetAuth(inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	client, err := auth.NetworkClient()
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := net.Context()

	req := ocicore.ListServicesRequest{}
	var items []map[string]interface{}
	truncated := false
	for page := 0; page < net.ListMaxPages; page++ {
		resp, err := client.ListServices(ctx, req)
		if err != nil {
			return net.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			svc := resp.Items[i]
			items = append(items, map[string]interface{}{
				"id":          net.Str(svc.Id),
				"name":        net.Str(svc.Name),
				"cidr_block":  net.Str(svc.CidrBlock),
				"description": net.Str(svc.Description),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == net.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d Oracle service(s) available in this region", len(items))
	if truncated {
		summary = fmt.Sprintf("Found at least %d Oracle service(s) (list truncated at %d pages — more available)", len(items), net.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"services":    items,
		"count":       len(items),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
