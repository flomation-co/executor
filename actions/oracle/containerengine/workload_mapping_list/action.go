// Package oracle_containerengine_workload_mapping_list lists the workload mappings on an OKE cluster.
package oracle_containerengine_workload_mapping_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: List Workload Mappings"
	Description  = "List the workload mappings on an Oracle Cloud OKE cluster. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa… — the OKE cluster to list mappings for", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "workload_mappings", Type: core.ConnectionTypeObject, Label: "Workload Mappings"},
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
	clusterID, err := oke.RequiredString("cluster_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	req := okesdk.ListWorkloadMappingsRequest{ClusterId: &clusterID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= oke.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkloadMappings(oke.Context(), req)
		if err != nil {
			return oke.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, oke.SummariseWorkloadMappingSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return oke.Result(fmt.Sprintf("Found %d workload mapping(s)", len(out)), map[string]interface{}{
		"workload_mappings": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
