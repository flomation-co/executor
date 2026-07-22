// Package oracle_containerengine_work_request_list lists the OKE work requests in a compartment.
package oracle_containerengine_work_request_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: List Work Requests"
	Description  = "List the Oracle Cloud OKE work requests in a compartment, optionally scoped to a cluster or resource. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
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
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "Only work requests for this cluster (optional)"},
	{Name: "resource_ocid", Type: core.ConnectionTypeString, Label: "Resource OCID", Placeholder: "Only work requests affecting this resource (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_requests", Type: core.ConnectionTypeObject, Label: "Work Requests"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	req := okesdk.ListWorkRequestsRequest{CompartmentId: &compartment}
	if cluster := oke.OptionalString("cluster_ocid", inputs); cluster != "" {
		req.ClusterId = &cluster
	}
	if resource := oke.OptionalString("resource_ocid", inputs); resource != "" {
		req.ResourceId = &resource
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= oke.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkRequests(oke.Context(), req)
		if err != nil {
			return oke.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, oke.SummariseWorkRequestSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return oke.Result(fmt.Sprintf("Found %d work request(s)", len(out)), map[string]interface{}{
		"work_requests": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
